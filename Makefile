NAME=toxiproxy
VERSION=$(shell cat VERSION)
DEB=pkg/$(NAME)_$(VERSION)_amd64.deb
GODEP_PATH=$(shell pwd)/Godeps/_workspace
ORIGINAL_PATH=$(shell echo $(GOPATH))
COMBINED_GOPATH=$(GODEP_PATH):$(ORIGINAL_PATH)

.PHONY: packages deb test linux darwin

all: deb linux darwin
deb: $(DEB)

darwin: tmp/build/toxiproxy-server-darwin-amd64 tmp/build/toxiproxy-client-darwin-amd64
linux: tmp/build/toxiproxy-server-linux-amd64 tmp/build/toxiproxy-client-linux-amd64

build:
	GOPATH=$(COMBINED_GOPATH) go build -o toxiproxy

clean:
	rm tmp/build/*
	rm *.deb

test:
	GOMAXPROCS=4 GOPATH=$(COMBINED_GOPATH) go test -v

tmp/build/toxiproxy-server-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -o $(@) github.com/Shopify/toxiproxy

tmp/build/toxiproxy-client-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -o $(@) github.com/Shopify/toxiproxy/cli

tmp/build/toxiproxy-server-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -o $(@) github.com/Shopify/toxiproxy

tmp/build/toxiproxy-client-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -o $(@) github.com/Shopify/toxiproxy/cli

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
