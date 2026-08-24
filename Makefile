BIN := bin/forestry

.PHONY: all build run test cover vet lint fmt install clean

all: lint test build

build:
	go build -o $(BIN) .

run:
	go run .

test:
	go test ./...

cover:
	go test -cover ./...

vet:
	go vet ./...

lint: vet
	test -z "$$(gofmt -l .)"
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

fmt:
	go fmt ./...

install:
	go install .

clean:
	rm -rf bin
