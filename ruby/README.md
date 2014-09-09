# Toxiproxy

```ruby
proxy = Toxiproxy::Proxy.create(
  upstream: "localhost:3306",
  name: "mysql_master",
  proxy: "localhost:22222"
)

Toxiproxy[:mysql_master]
# => Toxiproxy::Proxy

Toxiproxy[:mysql_master].state(:down) do
  TCPSocket.new("localhost", 22222)
  # raises Errno::ECONNREFUSED
end

# all good outside the block
TCPSocket.new("localhost", 22222)
```
