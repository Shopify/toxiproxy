# Toxiproxy

```ruby
proxy = Toxiproxy::Proxy.create(
  upstream: "localhost:3306",
  name: "mysql_master",
  proxy: "localhost:22222"
)

# proxies traffic through to mysql
TCPSocket.new("localhost", 22222)

proxy.destroy

TCPSocket.new("localhost", 22222)
# raises Errno::ECONNREFUSED

# we can now route traffic again
proxy.create

Toxiproxy[:mysql_master]
# => Toxiproxy::Proxy

Toxiproxy[:mysql_master].state(:down) do
  TCPSocket.new("localhost", 22222)
  # raises Errno::ECONNREFUSED
end

# all good now
TCPSocket.new("localhost", 22222)
```
