## Context

The dashboard's per-model breakdown aggregates captured requests with
`GROUP BY model, endpoint, upstream_host`. Every proxied request is persisted
with `endpoint = r.URL.Path`, including control-plane / helper calls that are
not model generation:

- `GET /models` (and `/v1/models`, `/copilot/models`) - OpenAI-compatible model
  listing. The proxy already special-cases this path for policy filtering
  (`internal/proxy/model_discovery.go`) but still persists the request as a
  usage row.
- `GET /agents` (and prefixed variants) - Copilot agent listing.
- Any other metadata / discovery GET that carries no `model` field and no token
  usage.

These rows are persisted with an empty model (rendered as `<unknown>`), zero
tokens, and zero cost, yet they inflate request counts and appear as rows in the
Models table, the timeline, and the CLI `stats` / `cost` / `today` output.

Analysis of the endpoint surface that flows through the proxy (grounded in the
OpenAI-compatible Copilot gateway plus reverse-engineered proxies):

- **Inference endpoints** (carry a `model` field and token usage):
  `/chat/completions`, `/completions`, `/responses`, `/embeddings`, `/messages`,
  with optional `/v1/`, `/copilot/`, or `/openai/` prefixes.
- **Control-plane / helper endpoints** (no model, no usage): model listings
  (`/models`), agent listings (`/agents`), and any other discovery / metadata
  GET.

The distinguishing signal between the two is robust: a control-plane request has
neither a model in its body nor any captured token usage. This holds for
`/models`, `/agents`, and any future discovery endpoint, so a single rule covers
"the others" without enumerating every helper path.

Built-in local paths `/_health` and `/_ping` are already handled before
persistence and are therefore out of scope.

## Goals / Non-Goals

**Goals:**

- Give every captured request a durable `endpoint_kind` of `inference` or
  `control_plane`.
- Make all model-usage views (dashboard Models breakdown, overview totals,
  projected cost, timeline; CLI `stats`, `cost`, `today`; reporting API
  `/api/stats`, `/api/cost`, `/api/today`, `/api/stats/timeline`) reflect only
  `inference` traffic by default.
- Preserve full-fidelity capture: every request is still stored, still visible
  in raw logs and anomaly detection, and still exported via CSV.

**Non-Goals:**

- Changing request forwarding, streaming, policy enforcement, or WebSocket
  handling.
- Changing session boundaries or session attribution (a helper GET still
  participates in session windows; revisiting session semantics is a separate
  change).
- Adding a UI toggle to show / hide control-plane traffic (can be added later if
  users ask for it).
- Classifying the privacy redaction surface or changing what is persisted for
  inference rows.

## Decisions

### Decision 1: Classify at capture time and persist `endpoint_kind`

Classify each request in the proxy before persistence and store the kind in a
new `endpoint_kind` column on the `requests` table.

**Rationale:** Persisting the kind makes it durable, queryable, and available to
every consumer (reporting, dashboard, export, anomalies) without recomputing it
at query time. It also survives future classification-rule changes because the
value is frozen at capture, matching how other captured flags
(`headroom_proxied`, `usage_missing`) are handled.

**Alternatives considered:**

- _Query-time classification via a SQL `CASE` over `endpoint`._ Rejected: pushes
  the rule into every aggregation query, duplicates logic across `Stats`, cost,
  timeline, today, and export, and cannot be indexed or audited per-row.
- _Do not persist; derive in the API layer._ Rejected: same duplication, and the
  CSV export would need its own copy of the rule.

### Decision 2: Hybrid classification - known helper anchors plus a model/usage signal

Classify a request as `control_plane` when, after normalizing a leading `/v1/`,
`/copilot/`, or `/openai/` prefix and a trailing slash, EITHER:

- the path is a known helper path (`/models`, `/agents`); OR
- the request has no model field in its body AND no captured token usage.

Otherwise classify as `inference`.

**Rationale:** This cleanly removes `/models` and `/agents` (the observed
helpers) and generalizes to any other discovery / metadata GET via the "no model
and no usage" signal, so new helper endpoints are handled without a code change.
At the same time it never silently drops genuine inference: an unknown path that
carries a model or usage is still counted as `inference`, protecting against a
future inference endpoint being misclassified as noise.

**Alternatives considered:**

- _Pure inference allowlist_ (`/chat/completions`, `/completions`, `/responses`,
  `/embeddings`, `/messages` only). Rejected as the sole rule: a new inference
  endpoint would be silently dropped from usage stats until the allowlist is
  updated. Kept as documentation of the known inference set.
- _Pure helper denylist_ (`/models`, `/agents` only). Rejected: does not
  generalize to other discovery endpoints; an unknown GET with no usage would
  still appear as a `<unknown>` usage row.

The exact path-normalization and matching helpers live in `internal/proxy` (near
the existing `isModelDiscoveryResponse` logic) and are unit-tested.

### Decision 3: Usage views are inference-only by default, with no new flag

Reporting and dashboard usage views filter to `endpoint_kind = 'inference'`
unconditionally. CSV export retains all rows.

**Rationale:** A usage monitor should report usage. Users asking for "request
count" mean model-generation requests, not how many times the client polled
`/models`. Adding a toggle now is speculative (YAGNI); the data is still
captured and exportable, so nothing is lost if a toggle is needed later.

**Alternatives considered:**

- _Add `--include-control-plane` / `?include_control_plane=1`._ Rejected for
  now: no user has asked for it and it expands the flag/param surface. Easy to
  add later without a schema change.

### Decision 4: CSV export keeps all rows and gains an `endpoint_kind` column

The export is full-fidelity request metadata, so it includes both kinds and adds
`endpoint_kind` to honor IA-001 / IA-006 (same name across CLI params, JSON
keys, and CSV columns).

### Decision 5: Additive schema migration

Add `endpoint_kind TEXT` (nullable) to `requests` and backfill existing rows
using the same classification rule on the persisted `endpoint` and a best-effort
signal from `model` / token columns. New rows always write the kind; the column
stays nullable so old binaries opening a migrated database do not break.

**Rationale:** Satisfies NFR-006 (no data loss on existing databases) and
NFR-008 (single binary). Nullable + backfill avoids a destructive migration.

## Risks / Trade-offs

- [An inference endpoint not yet known carries no model and no usage in some
  edge case] → It would be classified `control_plane` and dropped from usage
  stats. Mitigation: the "has model OR has usage" signal keeps any real
  generation counted; only truly usage-less traffic is excluded, which is
  correct for a usage monitor.
- [Existing databases need backfill] → Migration runs once at open; backfill is
  idempotent and only touches rows where `endpoint_kind IS NULL`. Rollback is
  simply ignoring the column.
- [Reported request counts drop after migration for users with lots of `/models`
  polling] → This is the intended correction, but it is a visible change.
  Mitigation: document it in the changelog and docs; raw counts remain available
  via CSV export.
- [Classification rule duplicated between capture-time Go code and the SQL
  backfill] → Keep the backfill SQL simple (path-prefix membership) and have the
  Go classifier be the source of truth for new rows; accept minor divergence for
  historical rows.

## Migration Plan

1. Add `endpoint_kind TEXT` column to `requests` (additive `ALTER TABLE`).
2. Backfill `endpoint_kind` for existing rows from `endpoint` (and, for
   ambiguous rows, from `model` / token presence).
3. New captures always set `endpoint_kind` via the proxy classifier.
4. Reporting queries (`Stats`, cost rows, timeline, today) add
   `AND endpoint_kind = 'inference'`; export adds the column without the filter.
5. Ship in one release; no data is deleted, so rollback (ignoring the column)
   restores prior reporting behavior.

## Open Questions

- Should session request counts also exclude control-plane traffic, or keep
  sessions as-is? Default: keep sessions as-is (out of scope) unless the user
  wants attribution corrected in the same change.
- Should `/embeddings` rows, which are inference but rarely cost-tracked in the
  catalog, remain in usage views? Default: yes, they are inference and stay.
