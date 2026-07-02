default: all

all: vet test build

build:
    go build ./cmd/copilot-monitor

test:
    go test ./...

vet:
    go vet ./...
    go run honnef.co/go/tools/cmd/staticcheck@latest ./...
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmt:
    go fmt ./...

clean:
    rm -f copilot-monitor phase0
