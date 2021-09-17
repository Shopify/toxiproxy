FROM scratch

EXPOSE 8474
ENTRYPOINT ["/toxiproxy"]
CMD ["-host=0.0.0.0"]

COPY toxiproxy-server-linux-* /toxiproxy
COPY toxiproxy-client-linux-* /toxiproxy-cli
