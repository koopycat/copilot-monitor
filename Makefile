.PHONY: build test vet fmt all clean

all: vet test build

build:
	go build ./cmd/copilot-monitor

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -f copilot-monitor phase0
