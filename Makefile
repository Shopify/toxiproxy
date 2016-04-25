SERVER_NAME=toxiproxy-server
CLI_NAME=toxiproxy-cli
VERSION=$(shell cat VERSION)
DEB=pkg/toxiproxy_$(VERSION)_amd64.deb
GODEP_PATH=$(shell pwd)/Godeps/_workspace
ORIGINAL_PATH=$(shell echo $(GOPATH))
COMBINED_GOPATH=$(GODEP_PATH):$(ORIGINAL_PATH)

.PHONY: packages deb test linux darwin windows

build:
	GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=git-$(shell git rev-parse --short HEAD)" -o $(SERVER_NAME) ./cmd
	GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=git-$(shell git rev-parse --short HEAD)" -o $(CLI_NAME) ./cli

all: deb linux darwin windows
deb: $(DEB)

darwin: tmp/build/$(SERVER_NAME)-darwin-amd64 tmp/build/$(CLI_NAME)-darwin-amd64
linux: tmp/build/$(SERVER_NAME)-linux-amd64 tmp/build/$(CLI_NAME)-linux-amd64
windows: tmp/build/$(SERVER_NAME)-windows-amd64.exe tmp/build/$(CLI_NAME)-windows-amd64.exe

release: all docker

clean:
	rm -f tmp/build/*
	rm -f $(SERVER_NAME)
	rm -f $(CLI_NAME)
	rm -f *.deb

test:
	GOMAXPROCS=4 GOPATH=$(COMBINED_GOPATH) go test -v -race ./...

tmp/build/$(SERVER_NAME)-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/$(SERVER_NAME)-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/$(SERVER_NAME)-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/$(CLI_NAME)-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

tmp/build/$(CLI_NAME)-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

tmp/build/$(CLI_NAME)-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

$(DEB): tmp/build/$(SERVER_NAME)-linux-amd64 tmp/build/$(CLI_NAME)-linux-amd64
	fpm -t deb \
		-s dir \
		-p tmp/build/ \
		--name "toxiproxy" \
		--version $(VERSION) \
		--license MIT \
		--no-depends \
		--no-auto-depends \
		--architecture amd64 \
		--maintainer "Simon Eskildsen <simon.eskildsen@shopify.com>" \
		--description "TCP proxy to simulate network and system conditions" \
		--url "https://github.com/Shopify/toxiproxy" \
		$(word 1,$^)=/usr/bin/$(SERVER_NAME) \
		$(word 2,$^)=/usr/bin/$(CLI_NAME) \
		./share/toxiproxy.conf=/etc/init/toxiproxy.conf

docker:
	docker build --tag="shopify/toxiproxy:$(VERSION)" .
	docker tag -f shopify/toxiproxy:$(VERSION) shopify/toxiproxy:latest
	docker push shopify/toxiproxy:$(VERSION)
	docker push shopify/toxiproxy:latest
