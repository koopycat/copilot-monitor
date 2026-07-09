default: all

all: vet test build dashboard-build

build:
    cd dashboard && pnpm build
    go build ./cmd/copilot-monitor

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

fmt:
    go fmt ./...

AIR := `go env GOPATH` / "bin" / "air"

watch:
    {{AIR}}

clean:
    rm -rf copilot-monitor tmp build-errors.log dashboard/dist
