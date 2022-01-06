OS := $(shell uname -s)
GO_VERSION := $(shell go version | cut -f3 -d" ")
GO_MINOR_VERSION := $(shell echo $(GO_VERSION) | cut -f2 -d.)
GO_PATCH_VERSION := $(shell echo $(GO_VERSION) | cut -f3 -d. | sed "s/^\s*$$/0/")
MALLOC_ENV := $(shell [ $(OS) = Darwin -a $(GO_MINOR_VERSION) -eq 17 -a $(GO_PATCH_VERSION) -lt 6 ] && echo "MallocNanoZone=0")

.PHONY: all
all: setup build test bench fmt lint

.PHONY: test
test:
	# NOTE: https://github.com/golang/go/issues/49138
	$(MALLOC_ENV) go test -v -race -timeout 1m ./...

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
