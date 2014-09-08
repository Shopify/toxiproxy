require 'minitest/unit'
require 'minitest/autorun'
require_relative "../lib/toxiproxy"

class MiniTest::Unit::TestCase
  def teardown
    Toxiproxy::Proxy.all.each(&:destroy)
  end

  def with_tcpserver(port: 20122, &block)
    thread = Thread.new {
      server = TCPServer.new(port)
      loop do
        client = server.accept
        client.close
      end
      server.close
    }

    yield(port)

    thread.kill
  end
end
