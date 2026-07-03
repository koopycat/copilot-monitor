default: all

all: vet test build

build:
    go build ./cmd/copilot-monitor

test:
    go test ./...

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
    rm -rf copilot-monitor tmp build-errors.log
