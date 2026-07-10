default: all

all: vet secrets test build dashboard-build

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

test:
    go test ./...

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

fmt:
    go fmt ./...

AIR := `go env GOPATH` / "bin" / "air"

watch:
    {{AIR}}

clean:
    rm -rf bin tmp build-errors.log dashboard/dist
