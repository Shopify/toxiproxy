FROM scratch

EXPOSE 8474
ENTRYPOINT ["/toxiproxy"]
CMD ["-host=0.0.0.0"]

ENV LOG_LEVEL=info

COPY toxiproxy-server-linux-* /toxiproxy
COPY toxiproxy-cli-linux-* /toxiproxy-cli
