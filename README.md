# Toxiproxy

Toxiproxy is a framework for simulating network conditions. It's made to work in
testing/CI environments as well as development. It consists of two parts: a Go
proxy that all network connections go through, as well as client libraries that
can apply toxic conditions (toxics) to the link.

### Ruby Example
See ruby gem here: [toxiproxy-ruby](https://github.com/Shopify/toxiproxy-ruby)

Adding 1000ms of latency to the mysql server's response:
```ruby
Toxiproxy[:mysql_master].downstream(:latency, latency: 1000).apply do
  Shop.first # this takes at least 1s
end
```

### Types of Toxics

#### latency

Add a delay to all data going through the proxy. The delay is equal to `latency` +/- `jitter`

Fields:
 - `enabled`: true/false
 - `latency`: time in milliseconds
 - `jitter`: time in milliseconds

#### slow_close

Delay the TCP socket from closing until `delay` has elapsed.

Fields:
 - `enabled`: true/false
 - `delay`: time in milliseconds

#### timeout

Stops all data from getting through, and close the connection after `timeout`
If `timeout` is 0, the connection won't close, and data will be delayed until the toxic is disabled.

Fields:
 - `enabled`: true/false
 - `timeout`: time in milliseconds

### HTTP Interface

#### Proxy Fields:
 - `name`: proxy name (string)
 - `listen`: listen address (string)
 - `upstream`: proxy upstream address (string)
 - `enabled`: true/false (defaults to true on creation)

All endpoints are JSON.

 - **GET /proxies** - List existing proxies
 - **POST /proxies** - Create a new proxy
 - **GET /toxics** - List existing proxies with toxics included
 - **GET /proxies/{proxy}** - Show the proxy with both its upstream and downstream toxics
 - **DELETE /proxies/{proxy}** - Delete an existing proxy
 - **POST /proxies/{proxy}/enable** - Enable a proxy and start listening
 - **POST /proxies/{proxy}/disable** - Disable a proxy so it refuses connections
 - **GET /proxies/{proxy}/upstream/toxics** - List upstream toxics
 - **GET /proxies/{proxy}/downstream/toxics** - List downstream toxics
 - **POST /proxies/{proxy}/upstream/toxics/{toxic}** - Update upstream toxic
 - **POST /proxies/{proxy}/downstream/toxics/{toxic}** - Update downstream toxic
 - **GET /reset** - Enable all proxies and disable all toxics

### Curl Example
```bash
$ curl -i -d '{"name": "redis", "upstream": "localhost:6379", "listen": "localhost:26379"}' localhost:8474/proxies
HTTP/1.1 201 Created
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:05:39 GMT
Content-Length: 71

{"name":"redis","listen":"127.0.0.1:26379","upstream":"localhost:6379"}
```
```bash
$ redis-cli -p 26379
127.0.0.1:26379> SET omg pandas
OK
127.0.0.1:26379> GET omg
"pandas"
```
```bash
$ curl -i localhost:8474/proxies
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:06:54 GMT
Content-Length: 81

{"redis":{"name":"redis","listen":"127.0.0.1:26379","upstream":"localhost:6379"}}
```
```bash
$ curl -i -d '{"enabled":true, "latency":1000}' localhost:8474/proxies/redis/downstream/toxics/latency
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:37:25 GMT
Content-Length: 42

{"enabled":true,"latency":1000,"jitter":0}
```
```bash
$ redis-cli -p 26379
127.0.0.1:26379> GET "omg"
"pandas"
(1.00s)
127.0.0.1:26379> DEL "omg"
(integer) 1
(1.00s)
```
```bash
$ curl -i -d '{"enabled":false}' localhost:8474/proxies/redis/downstream/toxics/latency
HTTP/1.1 200 OK
Content-Type: application/json
Date: Mon, 10 Nov 2014 16:39:49 GMT
Content-Length: 43

{"enabled":false,"latency":1000,"jitter":0}
```
```bash
$ redis-cli -p 26379
127.0.0.1:26379> GET "omg"
(nil)
```
```bash
$ curl -i -X DELETE localhost:8474/proxies/redis
HTTP/1.1 204 No Content
Date: Mon, 10 Nov 2014 16:07:36 GMT
```
```bash
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
