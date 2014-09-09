require_relative 'test_helper'

class ToxiproxyTest < MiniTest::Unit::TestCase
  def test_retrieve_proxy_by_name
    refute Toxiproxy["mysql_master"]
    create_proxy(upstream: "localhost:3306", name: "mysql_master", proxy: "localhost:22000")
    assert Toxiproxy["mysql_master"]
  end
end
