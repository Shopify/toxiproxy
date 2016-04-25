# toxiproxy-go

This is the Go client library for the
[Toxiproxy](https://github.com/shopify/toxiproxy) API. Please read the [usage
section in the Toxiproxy README](https://github.com/shopify/toxiproxy#usage)
before attempting to use the client.

This client is compatible with Toxiproxy 2.x, for the latest 1.x client see
[v1.2.1](https://github.com/Shopify/toxiproxy/tree/v1.2.1/client).

## Changes in Toxiproxy-go Client 2.x

In order to make use of the 2.0 api, and to make usage a little easier, the
client api has changed:

 - `client.NewProxy()` no longer accepts a proxy as an argument.
 - `proxy.Create()` is removed in favour of using `proxy.Save()`.
 - Proxies can be created in a single call using `client.CreateProxy()`.
 - `proxy.Disable()` and `proxy.Enable()` have been added to simplify taking
    down a proxy.
 - `proxy.ToxicsUpstream` and `proxy.ToxicsDownstream` have been merged into a
    single `ActiveToxics` list.
 - `proxy.Toxics()`` no longer requires a direction to be specified, and will
    return toxics for both directions.
 - `proxy.SetToxic()` has been replaced by `proxy.AddToxic()`,
   `proxy.UpdateToxic()`, and `proxy.RemoveToxic()`.

## Usage

For detailed API docs please [see the Godoc
documentation](http://godoc.org/github.com/Shopify/toxiproxy/client).

First import toxiproxy and create a new client:
```go
import "github.com/Shopify/toxiproxy/client"

client := toxiproxy.NewClient("localhost:8474")
```

You can then create a new proxy using the client:
```go
proxy := client.CreateProxy("redis", "localhost:26379", "localhost:6379")
```

For large amounts of proxies, they can also be created using a configuration file:
```go
var config []toxiproxy.Proxy
data, _ := ioutil.ReadFile("config.json")
json.Unmarshal(data, &config)
proxies, err = client.Populate(config)
```
```json
[{
  "name": "redis",
  "listen": "localhost:26379",
  "upstream": "localhost:6379"
}]
```

Toxics can be added as follows:
```go
// Add 1s latency to 100% of downstream connections
proxy.AddToxic("latency_down", "latency", "downstream", 1.0, toxiproxy.Attributes{
    "latency": 1000,
})

// Change downstream latency to add 100ms of jitter
proxy.UpdateToxic("latency_down", 1.0, toxiproxy.Attributes{
    "jitter": 100,
})

// Remove the latency toxic
proxy.RemoveToxic("latency_down")
```


The proxy can be taken down using `Disable()`:
```go
proxy.Disable()
```

When a proxy is no longer needed, it can be cleaned up with `Delete()`:
```go
proxy.Delete()
```

## Full Example

```go
import (
    "net/http"
    "testing"
    "time"

    "github.com/Shopify/toxiproxy/client"
    "github.com/garyburd/redigo/redis"
)

var toxiClient *toxiproxy.Client
var proxies map[string]*toxiproxy.Proxy

func init() {
    var err error
    toxiClient = toxiproxy.NewClient("localhost:8474")
    proxies, err = toxiClient.Populate([]toxiproxy.Proxy{{
        Name:     "redis",
        Listen:   "localhost:26379",
        Upstream: "localhost:6379",
    }})
    if err != nil {
        panic(err)
    }
    // Alternatively, create the proxies manually with
    // toxiClient.CreateProxy("redis", "localhost:26379", "localhost:6379")
}

func TestRedisBackendDown(t *testing.T) {
    proxies["redis"].Disable()
    defer proxies["redis"].Enable()

    // Test that redis is down
    _, err := redis.Dial("tcp", ":26379")
    if err == nil {
        t.Fatal("Connection to redis did not fail")
    }
}

func TestRedisBackendSlow(t *testing.T) {
    proxies["redis"].AddToxic("", "latency", "", 1, toxiproxy.Attributes{
        "latency": 1000,
    })
    defer proxies["redis"].RemoveToxic("latency_downstream")

    // Test that redis is slow
    start := time.Now()
    conn, err := redis.Dial("tcp", ":26379")
    if err != nil {
        t.Fatal("Connection to redis failed", err)
    }

    _, err = conn.Do("GET", "test")
    if err != nil {
        t.Fatal("Redis command failed", err)
    } else if time.Since(start) < 900*time.Millisecond {
        t.Fatal("Redis command did not take long enough:", time.Since(start))
    }
}

func TestEphemeralProxy(t *testing.T) {
    proxy, _ := toxiClient.CreateProxy("test", "", "google.com:80")
    defer proxy.Delete()

    // Test connection through proxy.Listen
    resp, err := http.Get("http://" + proxy.Listen)
    if err != nil {
        t.Fatal(err)
    } else if resp.StatusCode != 200 {
        t.Fatal("Proxy to google failed:", resp.StatusCode)
    }
}
```
