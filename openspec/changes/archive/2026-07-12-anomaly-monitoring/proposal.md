## Why

The proxy sits between VSCode's Copilot extension and GitHub's Copilot API on
every request. When GitHub changes their API (new endpoints, different response
formats, new auth requirements), the proxy is the first component to notice but
currently silently logs errors or returns 502 without surfacing what happened.
Without anomaly detection, a Copilot API change can silently degrade or break
the user experience until someone happens to look at logs or a bug report
arrives.

## What Changes

- Add a new `anomalies` SQLite table to store structured anomaly records.
- Instrument the proxy handler at key detection points: unrouted paths, SSE
  parse errors, missing auth headers, unknown upstream hosts, unknown Content-
  Types, and unknown WebSocket event types.
- Add a new `copilot-monitor inspect` CLI command that surfaces anomalies from
  the database with filtering and alerting capabilities.
- Add detection hooks in the SSE observer and WebSocket frame parser.
- The proxy's core forwarding behavior does not change. All existing capture,
  routing, and reporting continue unchanged.

## Capabilities

### New Capabilities

- `anomaly-detection`: Detect and record abnormal request/response patterns
  including unrouted paths, parse errors, auth anomalies, unknown upstreams, and
  new WebSocket event types.
- `anomaly-inspection`: CLI subcommand to surface and filter anomaly records
  from the database, with alert-on-match support for cron/script automation.

### Modified Capabilities

- (none -- all changes are additive, no existing SHALL requirements change)

## Impact

- `/internal/store/schema.sql`: New `anomalies` table.
- `/internal/store/store.go`: New `WriteAnomaly` and `QueryAnomalies` methods.
- `/internal/proxy/server.go`: Detection hooks in `ServeHTTP`.
- `/internal/proxy/sse.go`: Parse error recording in `SSEObserver`.
- `/internal/proxy/websocket.go`: Unknown event type recording.
- `/internal/cli/`: New `inspect.go` command.
- Database migration: existing databases gain a new table with no data loss.
