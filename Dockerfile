FROM golang:1.4 

ADD . /app
RUN cd /app && GOPATH=/app/Godeps/_workspace go build -o toxiproxy

EXPOSE 8474
ENTRYPOINT ["/app/toxiproxy"]
CMD ["-host=0.0.0.0"]
