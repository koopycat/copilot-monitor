VET_BIN := `go env GOPATH` / "bin"

all: vet test build

build:
    go build ./cmd/copilot-monitor

test:
    go test ./...

vet:
    go vet ./...
    {{VET_BIN}}/staticcheck ./...
    {{VET_BIN}}/govulncheck ./...; true

fmt:
    go fmt ./...

clean:
    rm -f copilot-monitor phase0
