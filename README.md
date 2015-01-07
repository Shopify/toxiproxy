# Toxiproxy

Toxiproxy is a framework for simulating network conditions. It's made
specifically to work in testing, CI and development environments, supporting
deterministic tampering with connections, but with support for randomized chaos
and customization. We've been successfully using it in all development and test
environments at Shopify since October 2014 for resiliency testing.

Toxiproxy usage consists of two parts. A TCP proxy written in Go (what this
repository contains) and a client communicating with the proxy over HTTP. You
configure your application to make all development connections go through
Toxiproxy and can then manipulate their health via HTTP. See [Usage](#usage)
below on how to set up your project.

For example, to add 1000ms of latency to the response of MySQL from the [Ruby
client](https://github.com/Shopify/toxiproxy-ruby):

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

While the examples in this README are currently in Ruby, there's nothing
stopping you from creating a client in any other language (see
[Clients](https://github.com/shopify/toxiproxy#Clients)).

## Why yet another chaotic TCP proxy?

The existing ones we found didn't provide the kind of dynamic API we needed for
integration and unit testing. Linux tools like `nc` and so on are not
cross-platform and require root, which makes them problematic in a test,
development and CI environment.

## Clients

* [toxiproxy-ruby](https://github.com/Shopify/toxiproxy-ruby)

[toxiproxy-ruby]: https://github.com/Shopify/toxiproxy-ruby

## Example

Let's walk through an example with a Rails application. Note that Toxiproxy is
in no way tied to Ruby, it's just been our first usecase and it's currently the
only language that has a client. You can see the full example at
[Sirupsen/toxiproxy-rails-example](https://github.com/Sirupsen/toxiproxy-rails-example).
To get started right away, jump down to [Usage](https://github.com/Shopify/toxiproxy#Usage).

For our popular blog, for some reason we're storing the tags for our posts in
Redis and the posts themselves in MySQL. We might have a `Post` class that
includes some methods to manipulate tags in a [Redis set](http://redis.io/commands#set):

```ruby
class Post < ActiveRecord::Base
  # Return an Array of all the tags.
  def tags
    TagRedis.smembers(tag_key)
  end

  # Add a tag to the post.
  def add_tag(tag)
    TagRedis.sadd(tag_key, tag)
  end

  # Remove a tag from the post.
  def remove_tag(tag)
    TagRedis.srem(tag_key, tag)
  end

  # Return the key in Redis for the set of tags for the post.
  def tag_key
    "post:tags:#{self.id}"
  end
end
```

We've decided that erroring while writing to the tag data store
(adding/removing) is OK. However, if the tag data store is down, we should be
able to see the post with no tags. We could simply rescue the
`Redis::CannotConnectError` around the `SMEMBERS` Redis call in the `tags`
method. Let's use Toxiproxy to test that.

We've already installed Toxiproxy and it's running on our machine, so we can
skip to step two. We add Redis to `config/toxiproxy.json` (see Usage below, step
2):

```json
[
  {
    "name": "toxiproxy_test_redis_tags",
    "listen": "127.0.0.1:22222",
    "upstream": "127.0.0.1:6379"
  }
]
```

To populate Toxiproxy when our application boots, to `config/boot.rb` we add:

```ruby
require 'toxiproxy'
Toxiproxy.populate(File.join(File.dirname(File.expand_path(__FILE__)), "/toxiproxy.json"))
```

Then in `config/environments/test.rb` we set the `TagRedis` to be a Redis client
that connects to Redis through Toxiproxy by adding this line:

```ruby
TagRedis = Redis.new(port: 22222)
```

All calls in the test environment now go through Toxiproxy. That means we can
add a unit test where we simulate a failure:

```ruby
test "should return empty array when tag redis is down when listing tags" do
  @post.add_tag "mammals"

  # Take down all Redises in Toxiproxy
  Toxiproxy[/redis/].down do
    assert_equal [], @post.tags
  end
end
```

The test fails with `Redis::CannotConnectError`. Perfect! Toxiproxy took down
the Redis successfully for the duration of the closure. Let's fix the `tags`
method to be resilient:

```ruby
def tags
  TagRedis.smembers(tag_key)
rescue Redis::CannotConnectError
  []
end
```

The tests pass! We now have a unit test that proves fetching the tags when Redis
is down returns an empty array, instead of throwing an exception. For full
coverage you should also write an integration test that wraps fetching the
entire blog post page when Redis is down.

Full example application is at
[Sirupsen/toxiproxy-rails-example](https://github.com/Sirupsen/toxiproxy-rails-example).

## Usage

Configuring a project to use Toxiproxy consists of four steps:

1. Installing Toxiproxy
2. Creating `config/toxiproxy.json`
3. Populating Toxiproxy
4. Using Toxiproxy

### 1. Installing Toxiproxy

**Linux**

See [`Releases`](https://github.com/Shopify/toxiproxy/releases) for the latest
binaries and system packages for your architecture.

**Ubuntu**

```bash
$ wget -O toxiproxy-1.0.0.deb https://github.com/Shopify/toxiproxy/releases/download/v1.0.0/toxiproxy_1.0.0_amd64.deb
$ sudo dpkg -i toxiproxy-1.0.0.deb
$ sudo service start toxiproxy
```

**OS X**

```bash
$ brew tap shopify/shopify
$ brew install toxiproxy
```

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
doesn't disrupt the connections from your server running in another environment.

Use ports outside the ephemeral port range to avoid random port conflicts it's
`32,768` to `61,000` on Linux by default, see
`/proc/sys/net/ipv4/ip_local_port_range`.

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

#### down

Bringing a service down is not technically a toxic in the implementation of
Toxiproxy. This is done by `POST`ing to `/proxies/{proxy}` and setting the
`enabled` field to `false`.

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

Toxiproxy listens for HTTP on port **8474**.

#### Proxy Fields:
 - `name`: proxy name (string)
 - `listen`: listen address (string)
 - `upstream`: proxy upstream address (string)
 - `enabled`: true/false (defaults to true on creation)

To change a proxy's name, it must be deleted and recreated.

Changing the `listen` or `upstream` fields will restart the proxy and drop any active connections.

If `listen` is specified with a port of 0, toxiproxy will pick an ephemeral port. The `listen` field
in the response will be updated with the actual port.

If you change `enabled` to `false`, it'll take down the proxy. You can switch it
back to `true` to reenable it.


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

* `make all`. Build Toxiproxy binaries and packages for all platforms. Requires
  to have Go compiled with cross compilation enabled on Linux and Darwin (amd64)
  as well as [`fpm`](https://github.com/jordansissel/fpm) in your `$PATH` to
  build the Debian package.
* `make test`. Run the Toxiproxy tests.
* `make darwin`. Build binary for Darwin.
* `make linux`. Build binary for Linux.
