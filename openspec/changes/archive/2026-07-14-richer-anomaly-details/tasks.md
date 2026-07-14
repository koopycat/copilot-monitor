## 1. Proxy Anomaly Enrichment

- [x] 1.1 Populate `Model` field from `meta.Model` on `unrouted_path` anomaly
      record in `server.go`
- [x] 1.2 Write descriptive `Detail` for `unrouted_path`:
      `"no route matched: <METHOD> <PATH> (model <MODEL>, provider <PROVIDER>)"`
      with graceful handling of empty model
- [x] 1.3 Populate `Model`, `Upstream` fields on `auth_missing` anomaly record
      in `server.go`
- [x] 1.4 Update anomaly test expectations for the new Detail format
- [x] 1.5 Run `just test` to verify no regressions
