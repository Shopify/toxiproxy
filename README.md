# Toxiproxy

Toxiproxy is a framework for simulating network conditions. It's made to work in
testing/CI environments as well as development. It consists of two parts: a Go
proxy that all network connections go through, as well as a client library that
can apply a condition to the link. The client library controls Toxiproxy through
an HTTP interface:

```bash
$ curl -i -d '{"Name": "redis", "Upstream": "localhost:6379"}'
localhost:8474/proxies
HTTP/1.1 201 Created
Content-Type: application/json
Date: Sun, 07 Sep 2014 23:38:53 GMT
Content-Length: 71

{"Name":"redis","Listen":"localhost:40736","Upstream":"localhost:6379"}

$ redis-cli -p 40736
127.0.0.1:53646> SET omg pandas
OK
127.0.0.1:53646> GET omg
"pandas"

$ curl -i localhost:8474/proxies
HTTP/1.1 200 OK
Content-Type: application/json
Date: Sun, 07 Sep 2014 23:39:16 GMT
Content-Length: 81

{"redis":{"Name":"redis","Listen":"localhost:40736","Upstream":"localhost:6379"}}

$ curl -i -X DELETE localhost:8474/proxies/redis
HTTP/1.1 204 No Content
Date: Sun, 07 Sep 2014 23:40:00 GMT

$ telnet localhost 53646
Trying 127.0.0.1...
telnet: Unable to connect to remote host: Connection refused

```
>>>>>>> more
