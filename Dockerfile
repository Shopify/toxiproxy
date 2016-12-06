FROM alpine

COPY tmp/toxiproxy /go/bin/toxiproxy
COPY tmp/toxiproxy-cli /go/bin/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
