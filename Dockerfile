FROM golang:1.7

COPY . /go/src/github.com/Shopify/toxiproxy

RUN go get -ldflags="-X github.com/Shopify/toxiproxy.Version=$(cat /go/src/github.com/Shopify/toxiproxy/VERSION)" github.com/Shopify/toxiproxy/cmd/toxiproxy && \
    go get github.com/Shopify/toxiproxy/cmd/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
