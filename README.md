# Toxiproxy

![](http://i.imgur.com/sOaNw0o.png)

Toxiproxy is a framework for simulating network conditions. It's made
specifically to work in testing, CI and development environments, supporting
deterministic tampering with connections, but with support for randomized chaos
and customization. **Toxiproxy is the tool you need to prove with tests that
your application doesn't have single points of failure.** We've been
successfully using it in all development and test environments at Shopify since
October, 2014. See our [blog post][blog] on resiliency for more information.

Toxiproxy usage consists of two parts. A TCP proxy written in Go (what this
repository contains) and a client communicating with the proxy over HTTP. You
configure your application to make all test connections go through Toxiproxy
and can then manipulate their health via HTTP. See [Usage](#usage)
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
[Clients](#clients)).

## Table of Contents

1. [Why yet another chaotic TCP proxy?](#why-yet-another-chaotic-tcp-proxy)
2. [Clients](#clients)
3. [Example](#example)
4. [Usage](#usage)
  1. [Installing](#1-installing-toxiproxy)
    1. [Upgrading from 1.x](#upgrading-from-toxiproxy-1x)
  2. [Populating](#2-populating-toxiproxy)
  3. [Using](#3-using-toxiproxy)
5. [Toxics](#toxics)
  1. [Latency](#latency)
  2. [Down](#down)
  3. [Bandwidth](#bandwidth)
  4. [Slow close](#slow_close)
  5. [Timeout](#timeout)
  6. [Slicer](#slicer)
6. [HTTP API](#http-api)
  1. [Proxy fields](#proxy-fields)
  2. [Toxic fields](#toxic-fields)
  3. [Endpoints](#endpoints)
7. [CLI example](#cli-example)
8. [FAQ](#frequently-asked-questions)
9. [Development](#development)

## Why yet another chaotic TCP proxy?

The existing ones we found didn't provide the kind of dynamic API we needed for
integration and unit testing. Linux tools like `nc` and so on are not
cross-platform and require root, which makes them problematic in test,
development and CI environments.

## Clients

* [toxiproxy-ruby](https://github.com/Shopify/toxiproxy-ruby)
* [toxiproxy-go](https://github.com/Shopify/toxiproxy/tree/master/client)
* [toxiproxy.net](https://github.com/mdevilliers/Toxiproxy.Net)
* [toxiproxy-php-client](https://github.com/ihsw/toxiproxy-php-client)
* [toxiproxy-node](https://github.com/dlion/toxiproxy-node)
* [toxiproxy-java](https://github.com/trekawek/toxiproxy-java)

## Example

Let's walk through an example with a Rails application. Note that Toxiproxy is
in no way tied to Ruby, it's just been our first use case and it's currently the
only language that has a client. You can see the full example at
[Sirupsen/toxiproxy-rails-example](https://github.com/Sirupsen/toxiproxy-rails-example).
To get started right away, jump down to [Usage](#usage).

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

Since we've already installed Toxiproxy and it's running on our machine, we can
skip to step 2. This is where we need to make sure Toxiproxy has a mapping for
Redis tags. To `config/boot.rb` (before any connection is made) we add:

```ruby
require 'toxiproxy'

Toxiproxy.populate([
  {
    name: "toxiproxy_test_redis_tags",
    listen: "127.0.0.1:22222",
    upstream: "127.0.0.1:6379"
  }
])
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
2. Populating Toxiproxy
3. Using Toxiproxy

### 1. Installing Toxiproxy

**Linux**

See [`Releases`](https://github.com/Shopify/toxiproxy/releases) for the latest
binaries and system packages for your architecture.

**Ubuntu**

```bash
$ wget -O toxiproxy-1.2.1.deb https://github.com/Shopify/toxiproxy/releases/download/v1.2.1/toxiproxy_1.2.1_amd64.deb
$ sudo dpkg -i toxiproxy-1.2.1.deb
$ sudo service toxiproxy start
```

**OS X**

```bash
$ brew tap shopify/shopify
$ brew install toxiproxy
```

**Windows**

Toxiproxy for Windows is available for download at https://github.com/Shopify/toxiproxy/releases/download/v1.2.1/toxiproxy-windows-amd64.exe

**Docker**

Toxiproxy is available on [Docker Hub](https://hub.docker.com/r/shopify/toxiproxy/).

```bash
$ docker pull shopify/toxiproxy
$ docker run -it shopify/toxiproxy
```

**Source**

If you have Go installed, you can build Toxiproxy from source using the make file:
```bash
$ make build
$ ./toxiproxy-server
```

#### Upgrading from Toxiproxy 1.x

In Toxiproxy 2.0 several changes were made to the API that make it incompatible with version 1.x.
In order to use version 2.x of the Toxiproxy server, you will need to make sure your client
library supports the same version. You can check which version of Toxiproxy you are running by
looking at the `/version` endpoint.

See the documentation for your client library for specific library changes. Detailed changes
for the Toxiproxy server can been found in [CHANGELOG.md](https://github.com/Shopify/toxiproxy/blob/master/CHANGELOG.md).

### 2. Populating Toxiproxy

When your application boots, it needs to make sure that Toxiproxy knows which
endpoints to proxy where. The main parameters are: name, address for Toxiproxy
to **listen** on and the address of the upstream.

Some client libraries have helpers for this task, which is essentially just
making sure each proxy in a list is created. Example from the Ruby client:

```ruby
# Make sure `shopify_test_redis_master` and `shopify_test_mysql_master` are
# present in Toxiproxy
Toxiproxy.populate([
  {
    name: "shopify_test_redis_master",
    listen: "127.0.0.1:22220",
    upstream: "127.0.0.1:6379"
  },
  {
    name: "shopify_test_mysql_master",
    listen: "127.0.0.1:24220",
    upstream: "127.0.0.1:3306"
  }
])
```

This code needs to run as early in boot as possible, before any code establishes
a connection through Toxiproxy. Please check your client library for
documentation on the population helpers.

Alternatively use the CLI to create proxies, e.g.:

```bash
toxiproxy-cli create shopify_test_redis_master -l localhost:26379 -u localhost:6379
```

We recommend a naming such as the above: `<app>_<env>_<data store>_<shard>`.
This makes sure there are no clashes between applications using the same
Toxiproxy.

For large application we recommend storing the Toxiproxy configurations in a
separate configuration file. We use `config/toxiproxy.json`.

Use ports outside the ephemeral port range to avoid random port conflicts.
It's `32,768` to `61,000` on Linux by default, see
`/proc/sys/net/ipv4/ip_local_port_range`.

### 3. Using Toxiproxy

To use Toxiproxy, you now need to configure your application to connect through
Toxiproxy. Continuing with our example from step two, we can configure our Redis
client to connect through Toxiproxy:

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

Or via the CLI:

```bash
toxiproxy-cli toxic add shopify_test_redis_master -t latency -a latency=1000
```

Please consult your respective client library on usage.

### Toxics

Toxics manipulate the pipe between the client and upstream. They can be added
and removed from proxies using the [HTTP api](#http-api). Each toxic has its own parameters
to change how it affects the proxy links.

For documentation on implementing custom toxics, see [CREATING_TOXICS.md](https://github.com/Shopify/toxiproxy/blob/master/CREATING_TOXICS.md)

#### latency

Add a delay to all data going through the proxy. The delay is equal to `latency` +/- `jitter`.

Attributes:

 - `latency`: time in milliseconds
 - `jitter`: time in milliseconds

#### down

Bringing a service down is not technically a toxic in the implementation of
Toxiproxy. This is done by `POST`ing to `/proxies/{proxy}` and setting the
`enabled` field to `false`.

#### bandwidth

Limit a connection to a maximum number of kilobytes per second.

Attributes:

 - `rate`: rate in KB/s

#### slow_close

Delay the TCP socket from closing until `delay` has elapsed.

Attributes:

 - `delay`: time in milliseconds

#### timeout

Stops all data from getting through, and closes the connection after `timeout`. If
`timeout` is 0, the connection won't close, and data will be delayed until the
toxic is removed.

Attributes:

 - `timeout`: time in milliseconds

#### slicer

Slices TCP data up into small bits, optionally adding a delay between each
sliced "packet".

Attributes:

 - `average_size`: size in bytes of an average packet
 - `size_variation`: variation in bytes of an average packet (should be smaller than average_size)
 - `delay`: time in microseconds to delay each packet by

### HTTP API

All communication with the Toxiproxy daemon from the client happens through the
HTTP interface, which is described here.

Toxiproxy listens for HTTP on port **8474**.

#### Proxy fields:

 - `name`: proxy name (string)
 - `listen`: listen address (string)
 - `upstream`: proxy upstream address (string)
 - `enabled`: true/false (defaults to true on creation)

To change a proxy's name, it must be deleted and recreated.

Changing the `listen` or `upstream` fields will restart the proxy and drop any active connections.

If `listen` is specified with a port of 0, toxiproxy will pick an ephemeral port. The `listen` field
in the response will be updated with the actual port.

If you change `enabled` to `false`, it will take down the proxy. You can switch it
back to `true` to reenable it.

#### Toxic fields:

 - `name`: toxic name (string, defaults to `<type>_<stream>`)
 - `type`: toxic type (string)
 - `stream`: link direction to affect (defaults to `downstream`)
 - `toxicity`: probability of the toxic being applied to a link (defaults to 1.0, 100%)
 - `attributes`: a map of toxic-specific attributes

See [Toxics](#toxics) for toxic-specific attributes.

The `stream` direction must be either `upstream` or `downstream`. `upstream` applies
the toxic on the `client -> server` connection, while `downstream` applies the toxic
on the `server -> client` connection. This can be used to modify requests and responses
separately.

#### Endpoints

All endpoints are JSON.

 - **GET /proxies** - List existing proxies and their toxics
 - **POST /proxies** - Create a new proxy
 - **GET /proxies/{proxy}** - Show the proxy with all its active toxics
 - **POST /proxies/{proxy}** - Update a proxy's fields
 - **DELETE /proxies/{proxy}** - Delete an existing proxy
 - **GET /proxies/{proxy}/toxics** - List active toxics
 - **GET /proxies/{proxy}/toxics/{toxic}** - Get an active toxic's fields
 - **POST /proxies/{proxy}/toxics/{toxic}** - Update an active toxic
 - **DELETE /proxies/{proxy}/toxics/{toxic}** - Remove an active toxic
 - **POST /reset** - Enable all proxies and remove all active toxics
 - **GET /version** - Returns the server version number

### CLI Example

```bash
$ toxiproxy-cli create redis -l localhost:26379 -u localhost:6379
Created new proxy redis
$ toxiproxy-cli list
Listen          Upstream        Name  Enabled Toxics
======================================================================
127.0.0.1:26379 localhost:6379  redis true    None

Hint: inspect toxics with `toxiproxy-client inspect <proxyName>`
```

```bash
$ redis-cli -p 26379
127.0.0.1:26379> SET omg pandas
OK
127.0.0.1:26379> GET omg
"pandas"
```

```bash
$ toxiproxy-cli toxic add redis -t latency -a latency=1000
Added downstream latency toxic 'latency_downstream' on proxy 'redis'
```

```bash
$ redis-cli -p 26379
127.0.0.1:26379> GET omg
"pandas"
(1.00s)
127.0.0.1:26379> DEL omg
(integer) 1
(1.00s)
```

```bash
$ toxiproxy-cli toxic remove redis -n latency_downstream
Removed toxic 'latency_downstream' on proxy 'redis'
```

```bash
$ redis-cli -p 26379
127.0.0.1:26379> GET omg
(nil)
```

```bash
$ toxiproxy-cli delete redis
Deleted proxy redis
```

```bash
$ redis-cli -p 26379
Could not connect to Redis at 127.0.0.1:26379: Connection refused
```

### Frequently Asked Questions

**How fast is Toxiproxy?** The speed of Toxiproxy depends largely on your hardware,
but you can expect a latency of *< 100µs* when no toxics are enabled. When running
with `GOMAXPROCS=4` on a Macbook Pro we acheived *~1000MB/s* throughput, and as high
as *2400MB/s* on a higher end desktop. Basically, you can expect Toxiproxy to move
data around at least as fast the app you're testing.

**Can Toxiproxy do randomized testing?** Many of the available toxics can be configured
to have randomness, such as `jitter` in the `latency` toxic. There is also a
global `toxicity` parameter that specifies the percentage of connections a toxic
will affect. This is most useful for things like the `timeout` toxic, which would
allow X% of connections to timeout.

**I am not seeing my Toxiproxy actions reflected for MySQL**. MySQL will prefer
the local Unix domain socket for some clients, no matter which port you pass it
if the host is set to `localhost`. Configure your MySQL server to not create a
socket, and use `127.0.0.1` as the host. Remember to remove the old socket
after you restart the server.

**Toxiproxy causes intermittent connection failures**. Use ports outside the
ephemeral port range to avoid random port conflicts. It's `32,768` to `61,000` on
Linux by default, see `/proc/sys/net/ipv4/ip_local_port_range`.

**Should I run a Toxiproxy for each application?** No, we recommend using the
same Toxiproxy for all applications. To distinguish between services we
recommend naming your proxies with the scheme: `<app>_<env>_<data store>_<shard>`.
For example, `shopify_test_redis_master` or `shopify_development_mysql_1`.

### Development

* `make`. Build a toxiproxy development binary for the current platform.
* `make all`. Build Toxiproxy binaries and packages for all platforms. Requires
  to have Go compiled with cross compilation enabled on Linux and Darwin (amd64)
  as well as [`fpm`](https://github.com/jordansissel/fpm) in your `$PATH` to
  build the Debian package.
* `make test`. Run the Toxiproxy tests.
* `make darwin`. Build binary for Darwin.
* `make linux`. Build binary for Linux.
* `make windows`. Build binary for Windows.

### Release

1. Update `CHANGELOG.md`
2. Bump `VERSION`
3. Change versions in `README.md`
4. Commit
5. Tag
6. `make release` to create binaries, packages and push new Docker image
7. Create [Github draft release](https://github.com/Shopify/toxiproxy/releases/new) against new tag and upload binaries and Debian package
8. [Bump version for Homebrew](https://github.com/Shopify/homebrew-shopify/blob/master/toxiproxy.rb#L9)


[blog]: https://engineering.shopify.com/17489072-building-and-testing-resilient-ruby-on-rails-applications
