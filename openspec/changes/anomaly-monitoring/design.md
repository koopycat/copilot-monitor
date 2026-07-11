## Context

The proxy forwards every Copilot API request from VSCode to GitHub and captures
usage metadata. When GitHub changes its API (endpoints, response formats, auth),
the proxy silently fails or logs errors. Users only discover breakage when
Copilot stops working or reports show gaps.

The proxy already has logging at key points (unrouted paths return 502, SSE
parse errors increment a counter) but these go to stderr and are lost when
stderr is not monitored. No structured persistence exists for these events.

The existing store package already handles schema migration and CRUD operations.
The existing CLI already has filterable commands
(`copilot-monitor stats --since 30d`, etc.) with composable output (`--json`).
Both patterns can be reused for anomaly inspection.

## Goals / Non-Goals

**Goals:**

- Persist structured anomaly records in SQLite for later inspection.
- Detect: unrouted paths, SSE parse errors, missing auth headers, unknown
  upstream hosts, unknown Content-Types, and unknown WebSocket event types.
- Provide a `copilot-monitor inspect` CLI command for filtering and surfacing
  anomalies with alert-on-match support.
- Deduplicate same-category, same-path anomalies within a cooldown window to
  avoid flooding the database with repeated hits of the same new endpoint.
- Zero performance regression on the proxy's hot response path.

**Non-Goals:**

- Anomaly detection on stored data (e.g., "cost spike compared to last week").
  This is about real-time request/response pattern anomalies only.
- Alerting infrastructure (email, Slack, webhooks). `--alert-on-any` exit codes
  enable external cron/script integration but the proxy does no sending.
- Dashboard integration. Anomalies are CLI-only in this change. Dashboard
  integration can be a follow-up.
- Rate-limiting or anomaly-based blocking. This is detection only.

## Decisions

### Decision 1: SQLite table for anomalies

Anomaly records go into a new `anomalies` table in the existing SQLite database,
not a separate file or log.

**Rationale:** The store already has a migration pattern, WAL mode for
concurrent access, and the CLI already reads from it. A separate file would
require a second open/close lifecycle, separate CLI flag, and no query
integration with the request data.

**Alternatives considered:**

- Separate JSONL file (rejected: no query capabilities, CLI needs to parse a
  different format)
- In-memory ring buffer (rejected: lost on restart, no historical view)

### Decision 2: Goroutine-based async writes with ring buffer

Anomaly recording uses a buffered channel feeding a single goroutine that writes
to SQLite via `WriteAnomaly`. The hot path calls a non-blocking channel send
with a fallback to increment an `anomaly_dropped` counter when the channel is
full.

**Rationale:** Must satisfy the non-negotiable constraint that anomaly detection
introduces zero latency on the response path. WAL mode in SQLite already handles
concurrent reads well, but a `WriteAnomaly` call on the hot path would still
acquire a write lock for the duration of the insert. Moving writes to a
dedicated goroutine decouples detection (hot path) from persistence
(background).

**Channel size:** 1024. At steady state this is oversized (anomalies are rare by
definition). It's sized for startup bursts when a new config is deployed.

### Decision 3: Deduplication via in-memory cooldown map

Before writing to the channel, the handler checks a `sync.Map` keyed by
`(category, path, detail_hash)`. If the key exists and its timestamp is within 5
minutes (the cooldown window), the anomaly is dropped. Otherwise, the key is
updated and the record is written.

**Rationale:** When Copilot adds a new endpoint like `/v1/responses`, the proxy
returns 502 for every request to that path. Without deduplication, a single new
endpoint hit 50 times in a minute produces 50 identical anomaly records. The
cooldown reduces this to one per 5-minute window.

**Alternatives considered:**

- SQL-based dedup (rejected: requires a DB read on the hot path)
- Daily aggregation (rejected: first occurrence of a new anomaly matters)

### Decision 4: `copilot-monitor inspect` CLI command

A new subcommand `inspect` mirrors the pattern of `stats`, `cost`, and
`sessions`. It queries the `anomalies` table with optional `--since`,
`-- category`, and `--severity` flags, and supports `--json` for
machine-readable output.

**Additional flag:** `--alert-on-any` causes the command to exit with code 1
when any anomalies match. This enables cron/systemd timer scripts.

### Decision 5: Detection hook placement

Each detection hook lives in the package that owns the data it inspects:

| Hook                    | Package | Location                                       |
| ----------------------- | ------- | ---------------------------------------------- |
| Unrouted path           | `proxy` | `ServeHTTP` after route match fails            |
| Auth header missing     | `proxy` | `ServeHTTP` before routing (for CAPI paths)    |
| Unknown upstream host   | `proxy` | `ServeHTTP` after route match (for new hosts)  |
| SSE parse error         | `proxy` | `SSEObserver.processJSON` when unmarshal fails |
| Unknown Content-Type    | `proxy` | `ServeHTTP` after response arrives             |
| WebSocket unknown event | `proxy` | `proxyWebSocket` frame parser                  |

All hooks use the same `RecordAnomaly` method on the Handler struct, which
handles deduplication and channel dispatch.

## Risks / Trade-offs

- **Risk:** Anomaly goroutine panics and loses all anomaly writes.
  **Mitigation:** Recover in the goroutine and log the panic. Anomaly recording
  is advisory, not critical -- the proxy continues forwarding.

- **Risk:** Deduplication hides the volume of a recurring anomaly.
  **Mitigation:** The anomaly record includes a `first_seen` and `last_seen`
  timestamp. The `inspect` command can show the time window. A future iteration
  could add a `count` field for occurrences within the window.

- **Risk:** New database table adds to migration surface area. **Mitigation:**
  `CREATE TABLE IF NOT EXISTS` pattern already used by existing tables. No data
  migration needed -- the table can be created alongside existing tables with
  zero data loss.

- **Risk:** Anomalies table grows unbounded over time. **Mitigation:** Retain
  all records for now (total anomaly volume is expected to be in the hundreds,
  not millions). If needed, a future `--prune-anomalies --older-than` flag can
  be added to the `inspect` command.
