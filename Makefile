.PHONY: all
all: setup build

.PHONY: test
test:
	go test -v -race ./...

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
