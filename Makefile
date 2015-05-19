NAME=toxiproxy
VERSION=$(shell cat VERSION)
DEB=pkg/$(NAME)_$(VERSION)_amd64.deb
GODEP_PATH=$(shell pwd)/Godeps/_workspace
ORIGINAL_PATH=$(shell echo $(GOPATH))
COMBINED_GOPATH=$(GODEP_PATH):$(ORIGINAL_PATH)

.PHONY: packages deb test linux darwin windows

build:
	GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=git-$(shell git rev-parse --short HEAD)" -o $(NAME) ./cmd

all: deb linux darwin windows
deb: $(DEB)

darwin: tmp/build/toxiproxy-server-darwin-amd64 tmp/build/toxiproxy-client-darwin-amd64
linux: tmp/build/toxiproxy-server-linux-amd64 tmp/build/toxiproxy-client-linux-amd64
windows: tmp/build/toxiproxy-server-windows-amd64.exe tmp/build/toxiproxy-client-windows-amd64.exe

release: all docker

clean:
	rm -f tmp/build/*
	#rm -f $(NAME)
	rm -f *.deb

test:
	GOMAXPROCS=4 GOPATH=$(COMBINED_GOPATH) go test -v -race ./...

tmp/build/toxiproxy-server-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/toxiproxy-server-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/toxiproxy-server-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cmd

tmp/build/toxiproxy-client-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

tmp/build/toxiproxy-client-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

tmp/build/toxiproxy-client-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(VERSION)" -o $(@) ./cli

$(DEB): tmp/build/toxiproxy-server-linux-amd64 tmp/build/toxiproxy-client-linux-amd64
	fpm -t deb \
		-s dir \
		--name "toxiproxy" \
		--version $(VERSION) \
		--license MIT \
		--no-depends \
		--no-auto-depends \
		--architecture amd64 \
		--maintainer "Simon Eskildsen <simon.eskildsen@shopify.com>" \
		--description "TCP proxy to simulate network and system conditions" \
		--url "https://github.com/Shopify/toxiproxy" \
		$(word 1,$^)=/usr/bin/toxiproxy \
		$(word 1,$^)=/usr/bin/toxiproxy-server \
		$(word 2,$^)=/usr/bin/toxiproxy-client \
		./share/toxiproxy.conf=/etc/init/toxiproxy.conf

docker:
	docker build --tag="shopify/toxiproxy:$(VERSION)" .
	docker tag -f shopify/toxiproxy:$(VERSION) shopify/toxiproxy:latest
	docker push shopify/toxiproxy:$(VERSION)
	docker push shopify/toxiproxy:latest
