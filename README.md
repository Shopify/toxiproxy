# Toxiproxy

Toxiproxy is a framework for simulating network conditions. It's made
specifically to work in testing, CI and development environments, supporting
deterministic tampering with connections, but with support for randomized chaos
and customization.

Toxiproxy usage consists of two parts. A TCP proxy written in Go (what this
repository contains) and a client communicating with the proxy over HTTP. You
configure your application to make all development connections go through
Toxiproxy and can then manipulate their health via HTTP. See [Usage](#usage)
below on how to set up your project.

For example, to add 1000ms of latency to the response of MySQL from the [Ruby
client](toxiproxy-ruby):

```ruby
Toxiproxy[:mysql_master].downstream(:latency, latency: 1000).apply do
  Shop.first # this takes at least 1s
end
```

To take down all Redis instances:

```ruby
Toxiproxy[/redis/].down do
  Shop.first # this will throw an exception
end
```

## Clients

* [toxiproxy-ruby](toxiproxy-ruby)

[toxiproxy-ruby]: https://github.com/Shopify/toxiproxy-ruby

## Usage

Configuring a project to use Toxiproxy consists of three steps:

1. Installing Toxiproxy
2. Creating `config/toxiproxy.json`
3. Populating Toxiproxy
4. Using Toxiproxy

### 1. Installing Toxiproxy

**Ubuntu**

```bash
$ wget -O toxiproxy-0.0.1.tar.gz https://github.com/shopify/toxiproxy/archive/v0.0.1.deb
$ sudo dpkg -i toxiproxy-0.0.1.tar.gz
```

**Unsupported**

Compile with `make build`, put the binary in your path create an `init` script.

### 2. Creating `config/toxiproxy.json`

In `config/toxiproxy.json` you specify the mappings of service upstreams (e.g.
MySQL or Redis) and an address for Toxiproxy to listen on that proxies to that
upstream. You should have a `config/toxiproxy.json` for each repository that
uses Toxiproxy:

```json
[
  {
    "name": "shopify_test_redis_master",
    "listen": "127.0.0.1:22220",
    "upstream": "127.0.0.1:6379" 
  },
  {
    "name": "shopify_test_mysql_master",
    "listen": "127.0.0.1:24220",
    "upstream": "127.0.0.1:3306"
  }
]
```

This is a subset's of Shopify's main application's `config/toxiproxy.json`, note the
convention of `<app_name>_<environment>_<service>_<shard>`. It's strongly
recommended to stick to this convention for the client libraries to work best,
easing debugging, making the endpoints discoverable and so that running tests
doesn't tamper with your development server.

Use ports outside the ephemeral port range to avoid random port conflicts it's
32768 to 61000 on Linux by default, see `/proc/sys/net/ipv4/ip_local_port_range`.

### 3. Populating Toxiproxy

With `config/toxiproxy.json` we need to feed it into Toxiproxy. Toxiproxy
doesn't know about files, so you cannot tell it about the configuration file.
This is to avoid problems like switching branches where the configuration is
different and managing a global configuration file, which is a mess. Instead,
when booting your application it's responsible for making sure all the proxies
from `config/toxiproxy.json` are in Toxiproxy. The clients libraries have
helpers for this task, for example in Ruby during the initialization of your
application:

```ruby
# Makes sure all proxies from `config/toxiproxy.json` are present in Toxiproxy
Toxiproxy.populate("./config/toxiproxy.json")
```

Please check your client library for documentation on the population helpers.

### 4. Using Toxiproxy

To use Toxiproxy, you now need to configure your application to connect through
Toxiproxy, for example to use the `config/toxiproxy.json` above we'd need to
configure our Redis client to connect through toxiproxy:

```ruby
# old straight to redis
redis = Redis.new(port: 6380)

# new through toxiproxy
redis = Redis.new(port: 22220)
```

Now you can tamper with it through the Toxiproxy API. In Ruby:

```ruby
redis = Redis.new(port: 22220)

Toxiproxy[:shopify_test_redis_master].downstream(:latency, latency: 1000).apply do
  redis.get("test") # will take 1s
end
```

Please consult your respective client library on usage.

### Toxics

Toxics manipulate the pipe between the client and upstream.

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

Stops all data from getting through, and close the connection after `timeout` If
`timeout` is 0, the connection won't close, and data will be delayed until the
toxic is disabled.

Fields:

 - `enabled`: true/false
 - `timeout`: time in milliseconds

### HTTP API

All communication with the Toxiproxy daemon from the client happens through the
HTTP interface, which is described here.

#### Proxy Fields:
 - `name`: proxy name* (string)
 - `listen`: listen address* (string)
 - `upstream`: proxy upstream address* (string)
 - `enabled`: true/false (defaults to true on creation)

 \* Changing these fields will restart the proxy and drop any connections. Proxy name is not editable.

All endpoints are JSON.

 - **GET /proxies** - List existing proxies
 - **POST /proxies** - Create a new proxy
 - **GET /toxics** - List existing proxies with toxics included
 - **GET /proxies/{proxy}** - Show the proxy with both its upstream and downstream toxics
 - **POST /proxies/{proxy}** - Update a proxy's fields
 - **DELETE /proxies/{proxy}** - Delete an existing proxy
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

### Development

* `make build`. Build the Toxiproxy binary.
* `make test`. Run the Toxiproxy tests.
* `make packages`. Build system packages, requires `fpm` in your `$PATH`.
