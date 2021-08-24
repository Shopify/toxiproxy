NAME=toxiproxy
SERVER_NAME=$(NAME)-server
CLI_NAME=$(NAME)-cli
VERSION=$(shell cat VERSION)
DEB=_output/$(NAME)_$(VERSION)_amd64.deb

.PHONY: build
build:
	BUILD_PLATFORMS=" " ./build/release

all: setup deb release

.PHONY: deb
deb: $(DEB)

.PHONY: darwin
darwin:
	BUILD_PLATFORMS="darwin/amd64" ./build/release

.PHONY: linux
linux:
	BUILD_PLATFORMS="linux/amd64" ./build/release

.PHONY: windows
windows:
	BUILD_PLATFORMS="windows/amd64" ./build/release

.PHONY: release
release: docker-release
	./build/release

.PHONY: clean
clean:
	rm -f _output/*

.PHONY: test
test:
	echo "Testing with" `go version`
	GOMAXPROCS=4 go test -v -race ./...

$(DEB): linux
	fpm -t deb \
		-s dir \
		-p _output/ \
		--name "$(NAME)" \
		--version $(VERSION) \
		--license MIT \
		--no-depends \
		--no-auto-depends \
		--architecture amd64 \
		--maintainer "Simon Eskildsen <simon.eskildsen@shopify.com>" \
		--description "TCP proxy to simulate network and system conditions" \
		--url "https://github.com/Shopify/toxiproxy" \
		_output/$(SERVER_NAME)-linux-amd64=/usr/bin/$(SERVER_NAME) \
		_output/$(CLI_NAME)-linux-amd64=/usr/bin/$(CLI_NAME) \
		./share/toxiproxy.conf=/etc/init/toxiproxy.conf

.PHONY: docker
docker: build
	docker build -f Dockerfile --tag="shopify/toxiproxy:git" _output/

.PHONY: docker-release
docker-release: linux
	docker build -f Dockerfile --rm=true --tag="shopify/toxiproxy:$(VERSION)" _output/
	docker tag shopify/toxiproxy:$(VERSION) shopify/toxiproxy:latest
	docker push shopify/toxiproxy:$(VERSION)
	docker push shopify/toxiproxy:latest

.PHONY: setup
setup:
	go mod download
	go mod tidy
