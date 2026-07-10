# Roadmap

## Phase 1 — CLI & Setup (P1, M — ~2 days)

### 1.1 Generic setup instructions for all tools

- Rename `configure-vscode` to `configure`
- Add `--tool pi|claude-code|openai|anthropic` flag
- Print environment variable instructions for each tool
- Files: `cli/configure.go`, `cli/root.go`

### 1.2 Update CLI text to reflect multi-tool scope

- "monitors LLM API usage" instead of "monitors GitHub Copilot model API usage"
- Files: `cli/root.go`, `README.md`, `PRODUCT.md`, `specs/product-requirements.md`

### 1.3 Live tail shows upstream context

- Add upstream host to live session display
- Per-upstream breakdown when multiple active
- Files: `cli/live.go`

---

## Phase 2 — API Parsing & Scope (P1, M — ~2 days)

### 2.1 Google Gemini usage parsing

- Add `parseGeminiUsage()` for `usageMetadata` format
- Files: `capture.go`, `capture_test.go`

### 2.2 Add `provider` field to persisted requests

- New column `provider TEXT` in schema
- Populate from route or upstream host
- Files: `schema.sql`, `store.go`, `server.go`, `api/`

### 2.3 Request correlation ID logging

- Optional `request_id TEXT` column
- Files: `schema.sql`, `store.go`, `server.go`

---

## Phase 3 — Catalog & Pricing (P2, S — ~2 days)

### 3.1 Catalog auto-update

- `copilot-monitor catalog-update` command
- Local override file at `~/.config/copilot-monitor/models.json`
- `--catalog` flag on `run` and `serve`
- Files: `cli/catalog.go`, `catalog.go`, `cli/run.go`, `cli/serve.go`

### 3.2 Model aliases

- Map tool-specific names to standard model IDs
- Files: `models.json`, `catalog.go`

### 3.3 More provider inference

- Add mistral and cohere to `inferProvider()`
- deepseek already done
- Files: `catalog.go`, `models.json`

---

## Phase 4 — Distribution (P2, M — ~2 days)

### 4.1 Homebrew formula

- Create `koopycat/homebrew-copilot-monitor` tap
- Auto-generate formula in CI
- Files: `.github/workflows/release.yml`, `README.md`

### 4.2 `go install` support

- Add to README download section
- Files: `README.md`

### 4.3 Systemd / Windows service examples

- Example unit files in `examples/systemd/`
- Files: `examples/systemd/copilot-monitor.service`, `README.md`

---

## Phase 5 — Dashboard Polish (P2, S — ~2 days)

### 5.1 Export improvements

- Add upstream_host and session_id to CSV export
- Files: `store.go`

### 5.2 Accessibility

- WCAG AA contrast check
- `prefers-reduced-motion` for chart transitions
- `aria-label` attributes, keyboard navigation
- Files: `dashboard/`

---

## Phase 6 — Observability (P2, S — ~1 day)

### 6.1 Health endpoint improvements

- Add `db_ok` and `db_size_bytes` to `/api/health`
- Files: `api/health.go`

### 6.2 Metrics endpoint

- Proxy metrics: requests proxied, errors, uptime
- Files: `api/metrics.go`

### 6.3 Graceful shutdown drain

- Drain in-flight requests before exit
- Files: `server.go`, `cli/run.go`
