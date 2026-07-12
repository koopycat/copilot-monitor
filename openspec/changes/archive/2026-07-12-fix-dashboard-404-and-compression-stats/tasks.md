## 1. Dashboard 404 fix

- [x] 1.1 Add `acceptsHTML(r *http.Request) bool` helper
- [x] 1.2 Update `combinedDashProxy` to check Accept header before falling
      through to dashboard

## 2. Compression stats fix

- [x] 2.1 Filter stats query to `compression_status = 'applied'` in `store.go`
- [x] 2.2 Filter sessions query same way in `sessions.go`
- [x] 2.3 Update integration test helper SQL and expected values

## 3. Tests

- [x] 3.1 Run full test suite with race detector
