## Tests with Toxiproxy

### Setup

```shell
$ brew install shopify/shopify/toxiproxy kind
$ kind create cluster --config=cluster.yml
$ kubectl --context kind-kind apply -f resources.yml
$ kubectl wait deploy postgres --for condition=available --timeout=5m
$ psql -h 127.0.0.1 -U postgres -c "DROP DATABASE IF EXISTS sample"
$ psql -h 127.0.0.1 -U postgres -c "CREATE DATABASE sample"
$ psql -h 127.0.0.1 -U postgres -c "CREATE DATABASE sample_test"
```

### Run

```shell
$ go run ./
```

### Test

```shell
$ go test -v .
$ go test -v . -run TestMultipleToxics
```
