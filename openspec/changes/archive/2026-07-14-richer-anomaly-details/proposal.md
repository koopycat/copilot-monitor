## Why

Anomalies for unrouted proxy paths currently record
`"no route matched for path"` as the detail for every occurrence regardless of
what model was requested or which provider was targeted. The `AnomalyRecord`
struct already has `Model`, `Endpoint`, and `Upstream` fields, but the
unrouted-path recording code leaves them empty. This makes the anomaly feed
useless -- 42 identical "no route matched" entries tell the user nothing about
_which model_ they need to configure a route for.

## What Changes

- Populate `Model` from `meta.Model` on unrouted-path anomaly records
- Make `Detail` descriptive and unique per path+model combination:
  `"no route matched: POST /v1/chat/completions (model gpt-5, provider openai)"`
- Apply the same Model/Endpoint enrichment to `auth_missing` anomaly records

## Capabilities

### Modified Capabilities

- `anomaly-detection`: Anomaly records for unrouted paths and missing auth SHALL
  include model, endpoint, and a descriptive detail string that uniquely
  identifies the path, model, and provider combination.

## Impact

- **Proxy code**: `internal/proxy/server.go` lines 182-196 -- populate `Model`,
  `Endpoint` fields and write descriptive `Detail`
- **Dedup behavior**: Because dedup hashes include `Detail`, different
  path+model combinations naturally become separate anomaly records instead of
  being collapsed into one
- **CLI output**: `copilot-monitor inspect` will show distinct model/path
  details in the sample column
- **Dashboard**: Anomaly feed will show actionable information about which
  models need routes
- **No breaking changes**: AnomalyRecord struct unchanged. Only the values
  written to its fields change
