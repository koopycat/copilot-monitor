default: all

all: vet secrets test build  # fast targets only (excludes integration, e2e)

# ── Setup ────────────────────────────────────────────────────────────────────

setup:
    mise install
    cd dashboard && pnpm install --frozen-lockfile
    @echo "Install go tools: go install honnef.co/go/tools/cmd/staticcheck@latest golang.org/x/vuln/cmd/govulncheck@latest"

# ── Build ────────────────────────────────────────────────────────────────────

# Full build: dashboard + Go binary
build: dashboard-build
    go build -o ./bin/copilot-monitor ./cmd/copilot-monitor

# Go-only fast build (skips dashboard, uses stub embed). Use during development.
build-go:
    GOTOOLCHAIN=go1.26.5 go build -tags nodashboard -o ./bin/copilot-monitor ./cmd/copilot-monitor

dashboard-build:
    cd dashboard && pnpm install --frozen-lockfile && pnpm build

# ── Test ─────────────────────────────────────────────────────────────────────

# Fast unit tests (excludes integration)
test:
    GOTOOLCHAIN=go1.26.5 go test -tags nodashboard ./internal/api/... ./internal/catalog/... ./internal/cli/... ./internal/compression/... ./internal/cost/... ./internal/log/... ./internal/policy/... ./internal/proxy/... ./internal/store/...

# Run all tests across every package
test-all:
    GOTOOLCHAIN=go1.26.5 go test -tags nodashboard ./...

# Run a single test by name pattern: just test-one TestStoreSessions
test-one pattern:
    go test -tags nodashboard -run '{{pattern}}' ./...

# Run all tests in a specific package: just test-pkg ./internal/store
test-pkg pkg:
    go test -tags nodashboard '{{pkg}}'

# HTTP-level integration tests (no browser)
integration:
    go test -tags testonly ./internal/integration/...

e2e:
    cd internal/e2e && pnpm test

# ── Check ────────────────────────────────────────────────────────────────────

vet:
    GOTOOLCHAIN=go1.26.5 go vet ./...
    GOTOOLCHAIN=go1.26.5 go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
    GOTOOLCHAIN=go1.26.5 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...

# Full CI check: vet + staticcheck + test (matches what CI runs)
check: vet
    just build-go
    GOTOOLCHAIN=go1.26.5 go vet -tags nodashboard ./...
    GOTOOLCHAIN=go1.26.5 go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 -tags nodashboard ./...
    GOTOOLCHAIN=go1.26.5 go test -tags nodashboard ./internal/api/... ./internal/catalog/... ./internal/cli/... ./internal/compression/... ./internal/cost/... ./internal/log/... ./internal/policy/... ./internal/proxy/... ./internal/store/...

dashboard-check:
    cd dashboard && pnpm check

# Fast pattern-based secret scan (pre-commit also runs this)
secrets:
    @devenv shell -- gitleaks detect --no-git

# Deep scan with live credential verification (requires network)
secrets-deep: secrets
    @devenv shell -- trufflehog git "file://$(pwd)" --only-verified --fail

# Lint markdown files (same checks as pre-commit markdownlint hook)
lint-md:
    markdownlint --dot '*.md' 'docs/*.md' 'specs/*.md' '.github/*.md'

# ── Format ───────────────────────────────────────────────────────────────────

fmt-go:
    go fmt ./...

# Format all code: Go (gofmt + goimports) + JS/CSS/Svelte/HTML/MD/JSON/YAML (prettier)
format: fmt-go
    git ls-files --cached --others --exclude-standard -z -- '*.go' | xargs -0 go run golang.org/x/tools/cmd/goimports@latest -w
    cd dashboard && pnpm exec prettier --plugin prettier-plugin-svelte --write '../**/*.{js,ts,svelte,css,html,json,md,yaml,yml}' '!../node_modules/**' '!../.devenv/**' '!../dashboard/dist/**' '!../go.sum' '!../pnpm-lock.yaml'

# ── Dev ──────────────────────────────────────────────────────────────────────

dashboard-dev:
    cd dashboard && pnpm dev

AIR := `go env GOPATH` / "bin" / "air"

watch:
    {{AIR}}

clean:
    rm -rf bin tmp build-errors.log dashboard/dist

# ── Demo ────────────────────────────────────────────────────────────────────

# Regenerate README demo GIFs from synthetic seed data.
# Requires: vhs, ttyd, ffmpeg, gifsicle (install once: brew install vhs ttyd ffmpeg gifsicle)
demo-gifs: build-go
    go run ./demo/seed/
    vhs demo/copilot-monitor.tape
    gifsicle -O3 --colors 32 -o demo/copilot-monitor.gif demo/copilot-monitor.gif
    sh demo/record-nolive.sh
    @echo "Demos regenerated:"
    @ls -lh demo/copilot-monitor.gif demo/copilot-monitor-nolive.gif
