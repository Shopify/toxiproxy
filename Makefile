.PHONY: all
all: setup build test bench fmt lint

.PHONY: test
test:
	go test -v -race -timeout 1m ./...

.PHONY: bench
bench:
	# TODO: Investigate why benchmarks require more sockets: ulimit -n 10240
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
	go build -ldflags="-s -w" -o ./dist/toxiproxy-server ./cmd/server
	go build -ldflags="-s -w" -o ./dist/toxiproxy-cli ./cmd/cli

.PHONY: release
release:
	goreleaser release --rm-dist

.PHONY: release-dry
release-dry:
	goreleaser release --rm-dist --skip-publish --skip-validate

.PHONY: release-test
release-test: test bench e2e release-dry
	bin/release_test

.PHONY: setup
setup:
	go mod download
	go mod tidy

dist:
	mkdir -p dist

.PHONY: clean
clean:
	rm -fr dist/*
