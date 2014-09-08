require 'minitest/unit'
require 'minitest/autorun'
require_relative "../lib/toxiproxy"

class MiniTest::Unit::TestCase
  def teardown
    Toxiproxy::Proxy.all.each { |proxy| proxy.send(:destroy) }
  end

  def create_proxy(**args)
    # Private method for which the API is not yet stable, so we're encouraging
    # the use of #state for now and ghettoing it in the tests.
    Toxiproxy::Proxy.new(**args).send(:create)
  end

  def delete_proxy(proxy)
    # Private method for same reason as #create_proxy
    name = proxy.name if proxy.respond_to?(:name)
    Toxiproxy[name].send(:destroy)
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
