FROM alpine:3.14.1

COPY toxiproxy-server-linux-amd64 /go/bin/toxiproxy
COPY toxiproxy-cli-linux-amd64 /go/bin/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
