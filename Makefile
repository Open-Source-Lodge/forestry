BIN := bin/forestry

.PHONY: all build test lint

all: lint test build

build:
	go build -o $(BIN) .

test:
	go test -race -cover ./...

lint:
	test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run go fmt ./..."; exit 1; }
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go mod tidy && git diff --exit-code go.mod go.sum
