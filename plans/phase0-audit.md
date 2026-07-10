# Phase 0 Audit: Copilot-Specific Assumptions Inventory

> Generated from exhaustive search across all source files (Go, SQL, JS, HTML, CSS, Svelte, Markdown, JSON, YAML, shell, justfile, Dockerfile), excluding `.devenv/` and `node_modules/`.

## Summary Counts

| Category | Count | Description |
|----------|-------|-------------|
| `routing` | 7 | Upstream hosts, path routing logic, error messages |
| `naming` | 17 | Binary name, brand labels, help text, UI copy, file paths |
| `cost` | 5 | Pricing assumptions, not-billed logic, cost output text |
| `capture` | 3 | Usage extraction logic, capture modes, WebSocket parsing |
| `schema` | 2 | DB schema field, pricing catalog source link |
| `docs/user-facing` | 14 | README, product spec, architecture docs, HTML landing page |
| `docs/internal` | 6 | AGENTS.md, code comments, inferProvider() |
| `build` | 9 | Module path, binary name, build scripts, package names |
| **Total** | **63** | |

---

## Critical vs Cosmetic

**Critical** (changing these without corresponding config changes breaks proxy routing or data integrity):
- Hardcoded upstream host constants (`GitHubCopilotAPIHost`, `GitHubCopilotProxyHost`)
- `copilotRoutePath()` function and all its path→upstream mappings
- Fallback calls to `copilotRoutePath()` in `Router.Match` and `Router.MatchModel`
- `"unknown Copilot path"` error message in `server.go`
- `isNotBilledEndpoint("completions")` — breaks cost calculation for non-Copilot providers
- `Endpoint` enum constants (used as keys in DB, routing, and cost)
- `inferProvider()` in `catalog.go` — incorrect provider assignment for generic models

**Cosmetic** (branding/labeling that can be changed freely):
- Binary name `copilot-monitor` and module path `copilot-monitoring`
- All help text, version output, startup log messages
- Dashboard title and footer text
- HTML landing page (`docs/index.html`) branding
- README, AGENTS.md, docs descriptions
- CSV export filename `copilot-export.csv`
- SQLite default paths containing `copilot-monitor`
- CLI command examples in help text
- `configure-vscode` command VSCode settings key `github.copilot.advanced`
- `justfile`, `.air.toml`, `.gitleaks.toml` references
- `dashboard/package.json` and `internal/e2e/package.json` names
- Pricing catalog source URL pointing to Copilot docs
- Code comments referencing "Copilot Responses API"
- All test data that uses `api.githubcopilot.com` as `UpstreamHost` (test data will need updating but doesn't affect production)

---

## Detailed Checklist

### routing

- [ ] `internal/proxy/router.go:10` — `GitHubCopilotAPIHost = "api.githubcopilot.com"` — hardcoded Copilot upstream host constant
- [ ] `internal/proxy/router.go:11` — `GitHubCopilotProxyHost = "copilot-proxy.githubusercontent.com"` — hardcoded Copilot proxy upstream host constant
- [ ] `internal/proxy/router.go:80` — `func copilotRoutePath(path string) (Route, bool)` — hardcoded Copilot path routing function
- [ ] `internal/proxy/router.go:85` — `Route{...Upstream: GitHubCopilotAPIHost...}` — chat completions path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:87` — `Route{...Upstream: GitHubCopilotAPIHost...}` — agents path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:89` — `Route{...Upstream: GitHubCopilotAPIHost...}` — models path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:91` — `Route{...Upstream: GitHubCopilotAPIHost...}` — models/session path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:93` — `Route{...Upstream: GitHubCopilotAPIHost...}` — responses path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:95` — `Route{...Upstream: GitHubCopilotAPIHost...}` — embeddings path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:97` — `Route{...Upstream: GitHubCopilotProxyHost...}` — engines completions path mapped to Copilot proxy upstream
- [ ] `internal/proxy/router.go:99` — `Route{...Upstream: GitHubCopilotProxyHost...}` — v1 completions path mapped to Copilot proxy upstream
- [ ] `internal/proxy/router.go:101` — `Route{...Upstream: GitHubCopilotAPIHost...}` — messages path mapped to Copilot upstream
- [ ] `internal/proxy/router.go:173` — `return copilotRoutePath(path)` — fallback to hardcoded Copilot routing in `Router.Match`
- [ ] `internal/proxy/router.go:195` — `return copilotRoutePath(path)` — fallback to hardcoded Copilot routing in `Router.MatchModel`
- [ ] `internal/proxy/server.go:79` — `"unknown Copilot path"` — error message for unmatched routes

### naming

- [ ] `internal/cli/root.go:22` — `"copilot-monitor %s\n"` — version output prints binary name
- [ ] `internal/cli/root.go:50` — `"copilot-monitor monitors GitHub Copilot model API usage..."` — help text description
- [ ] `internal/cli/root.go:53-62` — 10 command usage lines starting with `copilot-monitor`
- [ ] `internal/cli/serve.go:51` — `"copilot-monitor API listening on http://%s\n"` — startup log
- [ ] `internal/cli/run.go:84` — `"copilot-monitor listening on http://%s\n"` — startup log
- [ ] `internal/cli/configure.go:24` — `"github.copilot.advanced"` — VSCode settings JSON key
- [ ] `internal/cli/cost.go:62` — `"Estimated equivalent GitHub Copilot AI-credit list-price cost..."` — cost report header
- [ ] `internal/api/export.go:16` — `"copilot-export.csv"` — CSV export filename
- [ ] `internal/store/store.go:94` — `filepath.Join(xdg, "copilot-monitor", "store.db")` — XDG data path
- [ ] `internal/store/store.go:98` — `"copilot-monitor.db"` — fallback db filename
- [ ] `internal/store/store.go:100` — `filepath.Join(home, ".local", "share", "copilot-monitor", "store.db")` — home dir db path
- [ ] `dashboard/index.html:7` — `<title>Copilot Monitor</title>` — dashboard HTML page title
- [ ] `dashboard/src/App.svelte:19` — `Copilot Monitor` — dashboard Svelte component heading
- [ ] `dashboard/src/App.svelte:67` — `"Not your GitHub Copilot bill."` — dashboard footer text
- [ ] `internal/cli/cli_test.go:51` — test asserts stdout contains `"copilot-monitor"` — CLI test check
- [ ] `internal/cli/cli_test.go:97` — test asserts output contains `"Estimated equivalent GitHub Copilot AI-credit list-price cost"` — CLI test check
- [ ] `internal/e2e/tests/dashboard.spec.js:14` — test asserts `<h1>` contains `"Copilot Monitor"` — E2E test check

### cost

- [ ] `internal/cost/cost.go:57-58` — `isNotBilledEndpoint(stat.Endpoint)` — Copilot-specific billing logic
- [ ] `internal/cost/cost.go:62` — `isNotBilledEndpoint` function hardcodes `"completions"` as not billed
- [ ] `internal/cost/cost_test.go:43` — `TestCalculateCodeCompletionsAreNotBilled` — test assumes completions endpoint is free
- [ ] `internal/cost/cost.go:4-5` — import paths reference `copilot-monitoring/internal/...`
- [ ] `internal/cli/cost.go:62` — `"Estimated equivalent GitHub Copilot AI-credit list-price cost (%s). This is not your GitHub Copilot bill."` — cost output mentions Copilot

### capture

- [ ] `internal/proxy/router.go:85-101` — `CaptureUsage`, `CaptureNone`, `CaptureMetadata` assignments tied to Copilot `Endpoint` values
- [ ] `internal/proxy/capture.go:58` — comment: `"findNestedModel checks for the common Copilot Responses API nested path response.model"`
- [ ] `internal/proxy/websocket.go:191` — comment: `"model and usage data from Copilot Responses API events."`

### schema

- [ ] `internal/store/schema.sql:3` — `endpoint TEXT NOT NULL` — DB column stores `Endpoint` values (Copilot-derived enum)
- [ ] `internal/catalog/models.json:3` — `"source": "https://docs.github.com/de/copilot/reference/copilot-billing/models-and-pricing"` — pricing source is Copilot billing docs

### docs/user-facing

- [ ] `README.md:1` — `# Copilot Monitor` — project title
- [ ] `README.md:3-5` — CI and release badges reference `koopycat/copilot-monitor`
- [ ] `README.md:15` — `"Works with GitHub Copilot out of the box"`
- [ ] `README.md:21-28` — command examples `./copilot-monitor run` / `./copilot-monitor serve`
- [ ] `README.md:37-41` — download links with `copilot-monitor-{platform}` filenames
- [ ] `README.md:46-48` — install instructions with `copilot-monitor` binary
- [ ] `README.md:57-62` — `"GitHub Copilot picks models automatically... Copilot Monitor gives you the raw numbers"`
- [ ] `README.md:116-338` — command examples throughout with `./bin/copilot-monitor`
- [ ] `README.md:348` — `"~/.local/share/copilot-monitor/store.db"` — default db path
- [ ] `README.md:368-369` — `"GitHub Copilot AI-credit list-price estimate"` — cost disclaimer references Copilot
- [ ] `README.md:376` — Database path table entry `"~/.local/share/copilot-monitor/store.db"`
- [ ] `docs/index.html:6-1228` — entire landing page is Copilot-branded (title, headings, download links, flow diagrams, commands, repo URLs)
- [ ] `docs/api.md:3-7` — `"copilot-monitor serve"` / `"copilot-monitor run"` — API docs reference binary name
- [ ] `docs/architecture.md:3-74` — architecture doc describes Copilot-first routing
- [ ] `PRODUCT.md:18` — `"First-class built-in support for GitHub Copilot"`
- [ ] `specs/product-requirements.md:3-20` — PRD defines Copilot Monitor as GitHub Copilot-specific tool

### docs/internal

- [ ] `AGENTS.md:5` — `"Go CLI and local proxy for monitoring GitHub Copilot API usage"`
- [ ] `AGENTS.md:11` — `"just build builds ./cmd/copilot-monitor into ./copilot-monitor"`
- [ ] `AGENTS.md:20` — `"run ./copilot-monitor serve"` / `"run ./copilot-monitor run"`
- [ ] `internal/catalog/catalog.go:63-77` — `inferProvider()` — model-name-based provider guessing (Copilot-centric heuristic)
- [ ] `internal/proxy/capture.go:58` — code comment mentions Copilot Responses API
- [ ] `internal/proxy/websocket.go:191` — code comment mentions Copilot Responses API

### build

- [ ] `go.mod:1` — `module copilot-monitoring` — Go module name
- [ ] `cmd/copilot-monitor/main.go:1` — package path `cmd/copilot-monitor` — binary entry point directory
- [ ] `justfile:7` — `go build -o ./bin/copilot-monitor ./cmd/copilot-monitor` — build output binary name
- [ ] `.air.toml:9-12` — `bin = "./copilot-monitor"`, `cmd = "go build -o ./copilot-monitor ./cmd/copilot-monitor"` — air config
- [ ] `.gitleaks.toml:1,4` — `"copilot-monitor gitleaks config"`, `title = "copilot-monitor gitleaks config"`
- [ ] `dashboard/package.json:2` — `"name": "copilot-monitor-dashboard"` — npm package name
- [ ] `internal/e2e/package.json:4` — `"description": "End-to-end Playwright tests for copilot-monitor dashboard"`
- [ ] `internal/e2e/playwright.config.js:27` — `go run ../../cmd/copilot-monitor serve` — e2e server command
- [ ] 27 Go source files import `copilot-monitoring/...` (module path prefix) — bulk change on module rename

### Test data (cosmetic, will update alongside implementation)

- [ ] `internal/proxy/router_test.go:17-27` — 12 test cases reference `GitHubCopilotAPIHost`, `GitHubCopilotProxyHost`, `Endpoint*` constants
- [ ] `internal/proxy/router_test.go:304-305` — test asserts upstream equals `GitHubCopilotAPIHost`
- [ ] `internal/proxy/server_test.go:43-44,49-50,133-134` — tests assert upstream host equals `GitHubCopilotAPIHost`
- [ ] `internal/store/store_test.go:25-27` — test data uses `api.githubcopilot.com` and `copilot-proxy.githubusercontent.com`
- [ ] `internal/store/store_test.go:108-109` — test data uses `api.githubcopilot.com`
- [ ] `internal/store/sessions_test.go:20-22,57-58,90-91,131,176` — test data uses `api.githubcopilot.com`
- [ ] `internal/cli/cli_test.go:62,85,110,133,158-159,186-187,326,351,397,401` — test data uses `api.githubcopilot.com`
- [ ] `internal/api/api_test.go:28-29` — test data uses `api.githubcopilot.com`
- [ ] `internal/e2e/seed/main.go:124` — seed data uses `"api.githubcopilot.com"`
- [ ] `internal/proxy/router_test.go:24` — test path `"/v1/engines/copilot-codex/completions"` contains `copilot` in path

---

## Notes

- The `Endpoint` enum (`EndpointChat`, `EndpointAgent`, etc.) is used both as routing labels and as DB values in the `endpoint` column. Changing these requires a schema migration or backward-compatible mapping.
- `inferProvider()` in `catalog.go` uses model-name heuristics (e.g., `claude` → anthropic, `gpt-` → openai). This works for Copilot-transit models but will misclassify models from other providers.
- The `capture` category items are minor (comments only) — the actual capture logic is already generic and doesn't reference Copilot.
- 27 Go files import `copilot-monitoring/...` — a single module rename covers all of them.
