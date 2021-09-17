.PHONY: all
all: setup build test bench fmt lint

.PHONY: test
test:
	go test -v -race ./...

.PHONY: bench
bench:
	go test -bench=. -v *.go
	go test -bench=. -v toxics/*.go

.PHONY: fmt
fmt:
	go fmt ./...
	goimports -w **/*.go
	golangci-lint run --fix

.PHONY: lint
lint:
	golangci-lint run

.PHONY: e2e
e2e: build
	bin/e2e

.PHONY: build
build: dist clean
	go build -ldflags="-s -w" -o ./dist/toxiproxy-server ./cmd
	go build -ldflags="-s -w" -o ./dist/toxiproxy-cli ./cli

.PHONY: release
release:
	goreleaser release --rm-dist

.PHONY: setup
setup:
	go mod download
	go mod tidy

dist:
	mkdir -p dist

.PHONY: clean
clean:
	rm -fr dist/*
