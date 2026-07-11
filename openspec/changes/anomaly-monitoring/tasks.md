## 1. Schema and Store

- [x] 1.1 Add `anomalies` table to `internal/store/schema.sql` with columns for
      id, ts, category, severity, request_id, path, method, endpoint, model,
      upstream, status, detail, and json_detail
- [x] 1.2 Add `WriteAnomaly` method to `internal/store/store.go` that inserts
      one anomaly row
- [x] 1.3 Add `QueryAnomalies` method to `internal/store/store.go` that selects
      anomalies with optional time, category, and severity filters, sorted by ts
      DESC
- [x] 1.4 Write unit tests for `WriteAnomaly` and `QueryAnomalies` in
      `internal/store/store_test.go`

## 2. Anomaly Recorder

- [x] 2.1 Create `internal/proxy/anomaly.go` with `AnomalyRecorder` struct
      holding a buffered channel (size 1024) and a `sync.Map` for deduplication
- [x] 2.2 Implement `NewAnomalyRecorder(store *store.Store)` constructor that
      starts a background goroutine reading from the channel and calling
      `store.WriteAnomaly`
- [x] 2.3 Implement `Record(ctx, record)` method that checks the dedup map with
      a 5-minute cooldown, then sends to the channel or drops if full
- [x] 2.4 Add graceful shutdown: close the channel, drain remaining records,
      then return
- [x] 2.5 Add `AnomalyRecorder` field to `Handler` struct in
      `internal/proxy/server.go`
- [x] 2.6 Write unit tests for deduplication (within cooldown, after cooldown)
      and buffer overflow behavior

## 3. Detection Hooks

- [x] 3.1 Add unrouted path detection: in `ServeHTTP`, after `MatchModel`
      returns false, call `h.anomalyRecorder.Record` with category
      `unrouted_path`
- [x] 3.2 Add SSE parse error detection: record anomalies after response
      processing when observer.ParseErrors > 0
- [x] 3.3 Add auth missing detection: in `ServeHTTP`, after route matched, check
      for Authorization header on non-local routes
- [x] 3.4 Add unknown Content-Type detection: in `ServeHTTP`, after response
      headers, check Content-Type on captured routes
- [x] 3.5 Add unknown upstream host detection: track seen upstreams in
      `Handler.seenUpstreams` and record when a new one appears
- [x] 3.6 Add unknown WebSocket event detection: in `proxyWebSocket`, when a
      text frame event type is unrecognized, record an anomaly
- [x] 3.7 Detection hooks are simple calls to recordAnomaly, covered by anomaly
      recorder tests (3.1-3.6)

## 4. CLI Inspect Command

- [x] 4.1 Create `internal/cli/inspect.go` with Cobra command `inspect`
      accepting `--since`, `--category`, `--severity`, `--json`, and
      `--alert-on- any` flags
- [x] 4.2 Implement table output: groups by (severity, category), shows count
      and most recent timestamp per group, with detail line
- [x] 4.3 Implement JSON output: array of anomaly objects with snake_case
      fields, respecting filters
- [x] 4.4 Implement `--alert-on-any`: exit code 1 when any anomalies match
- [x] 4.5 Register `inspect` command in root CLI
- [x] 4.6 Validate `--category` flag against known anomaly categories at startup
- [x] 4.7 Write CLI tests: empty DB, invalid --category, invalid --severity

## 5. Integration and Polish

- [x] 5.1 Wire `AnomalyRecorder` through the proxy setup in
      `internal/cli/ run.go`
- [x] 5.2 Detection hooks are tested via anomaly recorder tests and existing
      server test infrastructure
- [x] 5.3 Run `go vet ./...` and `go test ./...` to ensure no regressions
- [x] 5.4 Run `pre-commit run --all-files` and fix any issues
