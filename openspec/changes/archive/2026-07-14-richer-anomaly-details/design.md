## Context

Proxy anomaly records for unrouted paths currently omit model, endpoint, and
upstream information despite these fields existing on the `AnomalyRecord`
struct. The `meta.Model` and `provider` variables are in scope at the recording
site in `server.go` but not used. This makes the anomaly feed non-actionable.

## Decisions

**Decision: Populate existing struct fields rather than adding new ones**

`AnomalyRecord` already has `Model`, `Endpoint`, `Upstream` fields. Populating
them is a data-only change -- no schema migration, no API changes, no UI changes
needed. The CLI `inspect` command and dashboard `AnomalyFeed` component already
render these fields when present.

**Decision: Make Detail unique per path+model combination**

The dedup mechanism hashes `Detail` as part of the dedup key. By including model
and provider in the Detail string
(`"no route matched: POST /v1/chat/completions (model gpt-5, provider openai)"`),
different path+model combinations naturally become separate anomaly records.
This is desirable because each distinct model that needs a route is a separate
actionable item.

**Decision: Handle empty model gracefully**

When `meta.Model` is empty (request body doesn't contain a model field), omit
the model portion: `"no route matched: GET /unknown"`.

## Risks / Trade-offs

- **[More anomaly records]** Previously all unrouted paths with the same path
  would dedup into one record. Now each distinct model gets its own. This is
  intentional -- 5 records for 5 different models on the same path is more
  useful than 1 record saying "something hit this path."
- **[No breaking changes]** Struct unchanged, schema unchanged, API unchanged.
