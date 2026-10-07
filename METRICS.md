# Metrics

- [Metrics](#metrics)
    - [Authentication](#authentication)
    - [Runtime Metrics](#runtime-metrics)
    - [Proxy Metrics](#proxy-metrics)
      - [toxiproxy_proxy_received_bytes_total / toxiproxy_proxy_sent_bytes_total](#toxiproxy_proxy_received_bytes_total--toxiproxy_proxy_sent_bytes_total)

### Authentication

When the server runs with `TOXIPROXY_AUTH_TOKEN` set, `/metrics` requires an
`Authorization: Bearer <token>` header like every other endpoint. For Prometheus, set
`authorization.credentials` (or `authorization.credentials_file`) in the scrape config.

### Runtime Metrics

To enable runtime metrics related to the state of the go runtime, build version, process info, use the `-runtime-metrics` flag.

For more details, see below:
- [NewGoCollector](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors#NewGoCollector)
- [NewBuildInfoCollector](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors#NewBuildInfoCollector)
- [NewProcessCollector](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors#NewProcessCollector)

### Proxy Metrics

To enable metrics related to toxiproxy internals, use the `-proxy-metrics` flag.
#### toxiproxy_proxy_received_bytes_total / toxiproxy_proxy_sent_bytes_total

The total number of bytes received/sent on a given proxy link in a given direction

```mermaid
sequenceDiagram
    Client->>+Toxiproxy: toxiproxy_proxy_received_bytes_total{direction="upstream"}
    Toxiproxy->>+Server: toxiproxy_proxy_sent_bytes_total{direction="upstream"}
    Server->>+Toxiproxy: toxiproxy_proxy_received_bytes_total{direction="downstream"}
    Toxiproxy->>+Client: toxiproxy_proxy_sent_bytes_total{direction="downstream"}
```

**Type**

Counter

**Labels**

| Label     | Description                    | Example               |
|-----------|--------------------------------|-----------------------|
| direction | Direction of the link          | upstream / downstream |
| listener  | Listener address of this proxy | 0.0.0.0:8080          |
| proxy     | Proxy name                     | my-proxy              |
| upstream  | Upstream address of this proxy | httpbin.org:80        |

