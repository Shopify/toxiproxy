FROM golang:1.7

COPY . /go/src/github.com/Shopify/toxiproxy

RUN go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(cat /go/src/github.com/Shopify/toxiproxy/VERSION)" -o /go/bin/toxiproxy github.com/Shopify/toxiproxy/cmd && \
    go build -o /go/bin/toxiproxy-cli github.com/Shopify/toxiproxy/cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
