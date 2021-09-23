.PHONY: all
all: setup build test bench fmt lint

.PHONY: test
test:
	go test -v -race -timeout 1m ./...

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

.PHONY: release-dry
release-dry:
	version="$(shell git describe --abbrev=0 --tags)"
	goreleaser build --rm-dist --single-target --skip-validate
	./dist/toxiproxy-cli-* --version | grep "toxiproxy-cli version $(version)"
	goreleaser release --rm-dist --skip-publish --skip-validate

.PHONY: setup
setup:
	go mod download
	go mod tidy

dist:
	mkdir -p dist

.PHONY: clean
clean:
	rm -fr dist/*
