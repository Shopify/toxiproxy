FROM golang:1.4

ADD . /app/src/github.com/Shopify/toxiproxy
RUN cd /app/src/github.com/Shopify/toxiproxy && GOPATH=/app/src/github.com/Shopify/toxiproxy/Godeps/_workspace:/app go build -o /app/toxiproxy ./cmd

EXPOSE 8474
ENTRYPOINT ["/app/toxiproxy"]
CMD ["-host=0.0.0.0"]
