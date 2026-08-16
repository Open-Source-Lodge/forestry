BIN := bin/forestry

.PHONY: all build run test cover vet fmt install clean

all: vet test build

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

fmt:
	go fmt ./...

install:
	go install .

clean:
	rm -rf bin
