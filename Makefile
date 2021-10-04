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
	goreleaser release --rm-dist --skip-publish --skip-validate

.PHONY: release-test
release-test: test bench e2e release-dry
	version="$(shell git describe --abbrev=0 --tags)"

	docker run -v $(PWD)/dist:/dist --pull always --rm -it ubuntu bash -c "dpkg -i /dist/toxiproxy_*_linux_amd64.deb; ls -1 /usr/bin/toxiproxy-*; /usr/bin/toxiproxy-cli --version | grep \"toxiproxy-cli version $(version)\""
	docker run -v $(PWD)/dist:/dist --pull always --rm -it centos bash -c "yum install -y /dist/toxiproxy_*_linux_amd64.rpm; ls -1 /usr/bin/toxiproxy-*; /usr/bin/toxiproxy-cli --version | grep \"toxiproxy-cli version $(version)\""
	docker run -v $(PWD)/dist:/dist --pull always --rm -it alpine sh -c "apk add --allow-untrusted --no-cache /dist/toxiproxy_*_linux_amd64.apk; ls -1 /usr/bin/toxiproxy-*; /usr/bin/toxiproxy-cli --version | grep \"toxiproxy-cli version $(version)\""

	tar -ztvf dist/toxiproxy_*_linux_amd64.tar.gz | grep toxiproxy-server
	tar -ztvf dist/toxiproxy_*_linux_amd64.tar.gz | grep toxiproxy-cli

	goreleaser build --rm-dist --single-target --skip-validate --id server
	./dist/toxiproxy-server-* --help 2>&1 | grep "Usage of ./dist/toxiproxy-server"

	goreleaser build --rm-dist --single-target --skip-validate --id client
	./dist/toxiproxy-cli-* --version | grep "toxiproxy-cli version $(version)"


.PHONY: setup
setup:
	go mod download
	go mod tidy

dist:
	mkdir -p dist

.PHONY: clean
clean:
	rm -fr dist/*
