# Toxiproxy

Toxiproxy is a framework for simulating network conditions. It's made to work in
testing/CI environments as well as development. It consists of two parts: a Go
proxy that all network connections go through, as well as client libraries that
can apply toxics (such as latency) to the link.

### Ruby Example
See ruby gem here: [toxiproxy-ruby](https://github.com/Shopify/toxiproxy-ruby)

Adding 1000ms of latency to the mysql server's response:
```ruby
Toxiproxy[:mysql_master].downstream(:latency, latency: 1000) do
  Shop.first # this takes at least 1s
end
```

### Types of Toxics

 - Latency
 - Slow Close
 - Timeout

### HTTP Interface

```bash
$ curl -i -d '{"name": "redis", "upstream": "localhost:6379", "listen": "localhost:26379"}'
localhost:8474/proxies
HTTP/1.1 201 Created
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:05:39 GMT
Content-Length: 71

{"name":"redis","listen":"127.0.0.1:26379","upstream":"localhost:6379"}

$ redis-cli -p 26379
127.0.0.1:26379> SET omg pandas
OK
127.0.0.1:26379> GET omg
"pandas"

$ curl -i localhost:8474/proxies
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:06:54 GMT
Content-Length: 81

{"redis":{"name":"redis","listen":"127.0.0.1:26379","upstream":"localhost:6379"}}

$ curl -i -X DELETE localhost:8474/proxies/redis
HTTP/1.1 204 No Content
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:07:36 GMT


$ redis-cli -p 26379
Could not connect to Redis at 127.0.0.1:26379: Connection refused
```

### Building

To compile:

`script/compile`

To build and rename for release (makes sure it has the right name and is cross
compiled for 64 bit Linux):

`script/build`

To release upload to Vagrant bucket in the Toxiproxy directory, with the name
from `script/build`.
