FROM alpine:3.11.11
ARG TARGETARCH

COPY tmp/build/toxiproxy-server-linux-$TARGETARCH /go/bin/toxiproxy
COPY tmp/build/toxiproxy-cli-linux-$TARGETARCH /go/bin/toxiproxy-cli

EXPOSE 8474
ENTRYPOINT ["/go/bin/toxiproxy"]
CMD ["-host=0.0.0.0"]
