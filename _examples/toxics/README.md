## Create Toxic

Example how to start building own toxics.

### Debug toxic

Run custom toxiserver with DebugToxic.

```shell
$ go run debug_toxic.go
```

Run redis-server in separate terminal:

```shell
$ redis-server
```

Test toxic with:

```shell
$ toxiproxy-cli --host "http://localhost:8484" create -l :16379 -u localhost:6379 redis
$ toxiproxy-cli --host "http://localhost:8484" toxic add --type debug redis
$ redis-cli -p 16379 "keys" "*"
```

Custom Toxiproxy should print bytes in hex format.

### HTTP toxic

Run custom toxiserver with DebugToxic.

```shell
$ go run http_toxic.go
```

Test toxic with command and verify output:

```shell
$ toxiproxy-cli --host "http://localhost:8484" create -l :18080 -u example.com:80 example
$ toxiproxy-cli --host "http://localhost:8484" toxic add --type http example
$ curl -v localhost:18080/hello
...
< HTTP/1.1 404 Not Found
< Location: https://github.com/Shopify/toxiproxy
```
