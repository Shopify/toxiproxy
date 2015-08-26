NAME=toxiproxy
VERSION=$(shell cat VERSION)
DEB=pkg/$(NAME)_$(VERSION)_amd64.deb
GODEP_PATH=$(shell pwd)/Godeps/_workspace
ORIGINAL_PATH=$(shell echo $(GOPATH))
COMBINED_GOPATH=$(GODEP_PATH):$(ORIGINAL_PATH)

.PHONY: packages deb test linux darwin windows

build:
	GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version git-`git rev-parse --short HEAD`" -o toxiproxy ./cmd

all: deb linux darwin windows
deb: $(DEB)
darwin: tmp/build/toxiproxy-darwin-amd64
linux: tmp/build/toxiproxy-linux-amd64
windows: tmp/build/toxiproxy-windows-amd64.exe
release: all docker

clean:
	rm tmp/build/*
	rm *.deb

test:
	GOMAXPROCS=4 GOPATH=$(COMBINED_GOPATH) go test -v -race ./...

tmp/build/toxiproxy-linux-amd64:
	GOOS=linux GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version $(VERSION)" -o $(@) ./cmd

tmp/build/toxiproxy-darwin-amd64:
	GOOS=darwin GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version $(VERSION)" -o $(@) ./cmd

tmp/build/toxiproxy-windows-amd64.exe:
	GOOS=windows GOARCH=amd64 GOPATH=$(COMBINED_GOPATH) go build -ldflags="-X github.com/Shopify/toxiproxy.Version $(VERSION)" -o $(@) ./cmd

docker:
	docker build --tag="shopify/toxiproxy:$(VERSION)" .
	docker tag -f shopify/toxiproxy:$(VERSION) shopify/toxiproxy:latest
	docker push shopify/toxiproxy:$(VERSION)
	docker push shopify/toxiproxy:latest

$(DEB): tmp/build/toxiproxy-linux-amd64
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
		$<=/usr/bin/toxiproxy \
		./share/toxiproxy.conf=/etc/init/toxiproxy.conf
