## ADDED Requirements

### Requirement: Compression aggregates filter to applied only

Compression statistics in dashboards and reports SHALL only count requests where
`compression_status` is `applied`. Requests with status `no_change`, `bypassed`,
or `failed_*` SHALL be excluded from compression aggregates
(`compressed_requests`, `compression_removed_tokens`, `avg_compression_ratio`).

#### Scenario: No-change requests excluded from stats

- **WHEN** a model has 10 `applied` requests (100 tokens saved) and 90
  `no_change` requests (0 tokens saved)
- **THEN** the stats report 10 compressed requests and 100 tokens removed
