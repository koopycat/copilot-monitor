default: all

all: vet secrets test build dashboard-build  # fast targets only (excludes integration, e2e)

setup:
    mise install
    cd dashboard && pnpm install --frozen-lockfile
    @echo "Install go tools: go install honnef.co/go/tools/cmd/staticcheck@latest golang.org/x/vuln/cmd/govulncheck@latest"

build:
    cd dashboard && pnpm build
    go build -o ./bin/copilot-monitor ./cmd/copilot-monitor

dashboard-build:
    cd dashboard && pnpm install --frozen-lockfile && pnpm build

dashboard-check:
    cd dashboard && pnpm check

dashboard-dev:
    cd dashboard && pnpm dev

# Excludes ./internal/integration/... by design — integration tests are slower
# and may need external setup. Run them explicitly with `just integration`.
test:
    go test $(go list ./... | grep -v 'internal/integration')

# HTTP-level integration tests (no browser)
integration:
    go test -tags testonly ./internal/integration/...

e2e:
    cd internal/e2e && pnpm test

vet:
    go vet ./...
    go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
    go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...

# Fast pattern-based secret scan (pre-commit also runs this)
secrets:
    @devenv shell -- gitleaks detect --no-git

# Deep scan with live credential verification (requires network)
secrets-deep: secrets
    @devenv shell -- trufflehog git "file://$(pwd)" --only-verified --fail

fmt-go:
    go fmt ./...

# Format all code: Go (gofmt + goimports) + JS/CSS/Svelte/HTML/MD/JSON/YAML (prettier)
format: fmt-go
    go run golang.org/x/tools/cmd/goimports@latest -w .
    cd dashboard && pnpm exec prettier --plugin prettier-plugin-svelte --write '../**/*.{js,ts,svelte,css,html,json,md,yaml,yml}' '!../node_modules/**' '!../.devenv/**' '!../dashboard/dist/**' '!../go.sum' '!../pnpm-lock.yaml'

AIR := `go env GOPATH` / "bin" / "air"

watch:
    {{AIR}}

clean:
    rm -rf bin tmp build-errors.log dashboard/dist
