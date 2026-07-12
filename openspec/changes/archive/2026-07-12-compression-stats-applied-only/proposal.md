## Why

The compression statistics in the dashboard and CLI include `no_change` rows in
averages. A `no_change` row means Headroom decided not to compress (ratio =
1.0). When most requests are under 500 tokens and thus `no_change`, the average
ratio approaches 1.0, displaying as "-99%" or "-100%" — misleading users into
thinking compression does nothing, when it actually saves 5-50% on the requests
it does compress.

## What Changes

- SQL aggregates for compression stats filter to
  `compression_status = 'applied'` only
- `compressed_requests` count only includes rows where tokens were actually
  reduced
- `compression_original_tokens`, `compression_final_tokens`,
  `compression_removed_tokens`, and `avg_compression_ratio` all use the same
  filter
- Dashboard "Token Reduction" column now reflects actual savings on compressed
  requests
- CLI `stats` and `live` commands show the same corrected numbers

## Capabilities

### Modified Capabilities

- `compression`: The status labels spec already says `applied`, `no_change`,
  etc. are distinct — no change to that. The change is in how aggregates are
  computed: only `applied` rows count.

## Impact

- `internal/store/store.go`: Stats query — filter compression aggregates to
  `applied` only
- `internal/store/sessions.go`: Sessions query — same filter
- `internal/integration/compression_test.go`: Update expected values
- No dashboard or API changes needed (they consume the corrected SQL output)
