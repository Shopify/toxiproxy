FROM alpine

COPY tmp/build/toxiproxy-server-linux-amd64 /go/bin/toxiproxy
COPY tmp/build/toxiproxy-cli-linux-amd64 /go/bin/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
