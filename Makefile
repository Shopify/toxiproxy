OS := $(shell uname -s)
ARCH := $(shell uname -m)
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

.PHONY: test-e2e
test-e2e: build container.build
	scripts/test-e2e
	timeout -v --foreground 20m scripts/test-e2e-hazelcast toxiproxy

.PHONY: test-release
test-release: lint fmt test bench test-e2e release-dry
	scripts/test-release

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
	shfmt -l -s -w -kp -i 2 scripts/test-*

.PHONY: lint
lint:
	golangci-lint run
	shellcheck scripts/test-*
	shfmt -l -s -d -kp -i 2 scripts/test-*
	yamllint .

.PHONY: build
build: dist clean
	go build -ldflags="-s -w" -o ./dist/toxiproxy-server ./cmd/server
	go build -ldflags="-s -w" -o ./dist/toxiproxy-cli ./cmd/cli

.PHONY: container.build
container.build:
	env GOOS=linux CGO_ENABLED=0 go build -ldflags="-s -w" -o ./dist/toxiproxy-server-linux-$(ARCH) ./cmd/server
	env GOOS=linux CGO_ENABLED=0 go build -ldflags="-s -w" -o ./dist/toxiproxy-cli-linux-$(ARCH) ./cmd/cli
	docker build -f Dockerfile -t toxiproxy dist
	docker run --rm toxiproxy --version

.PHONY: release
release:
	goreleaser release --rm-dist

.PHONY: release-dry
release-dry:
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
