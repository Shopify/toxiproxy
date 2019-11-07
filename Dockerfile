# Alpine 3.10.3
FROM alpine@sha256:e4355b66995c96b4b468159fc5c7e3540fcef961189ca13fee877798649f531a

COPY tmp/build/toxiproxy-server-linux-amd64 /go/bin/toxiproxy
COPY tmp/build/toxiproxy-cli-linux-amd64 /go/bin/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
