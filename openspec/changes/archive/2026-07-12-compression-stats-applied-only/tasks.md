## 1. SQL changes

- [x] 1.1 Update `Stats()` query in `internal/store/store.go`: filter
      compression aggregates to `compression_status = 'applied'` only
- [x] 1.2 Update session model query in `internal/store/sessions.go:238`: same
      filter

## 2. Tests

- [x] 2.1 Update `TestCompression_StatsAggregation` expected values (applied +
      no_change → applied only)
- [x] 2.2 Update `TestCompression_SessionModelStats` expected values if affected
- [x] 2.3 Run full test suite with race detector
