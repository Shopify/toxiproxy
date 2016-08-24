FROM golang:1.7

ADD . /app/src/github.com/Shopify/toxiproxy

ENV GOPATH /app:$GOPATH
ENV PATH $PATH:/app
WORKDIR /app/src/github.com/Shopify/toxiproxy
RUN go build -ldflags="-X github.com/Shopify/toxiproxy.Version=$(cat VERSION)" -o /app/toxiproxy ./cmd
RUN go build -o /app/toxiproxy-cli ./cli

EXPOSE 8474
ENTRYPOINT ["/app/toxiproxy"]
CMD ["-host=0.0.0.0"]
