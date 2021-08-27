.PHONY: all
all: setup build

.PHONY: test
test:
	go test -v -race ./...

.PHONY: build
build: clean
	goreleaser build --snapshot --rm-dist --skip-post-hooks --skip-validate

.PHONY: release
release:
	goreleaser release --rm-dist

.PHONY: clean
clean:
	rm -fr dist/*

.PHONY: setup
setup:
	go mod download
	go mod tidy
