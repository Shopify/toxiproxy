require "toxiproxy/version"
require "json"

module Toxiproxy
  class Proxy
    attr_reader :upstream, :proxy, :name

    def initialize(upstream:, name:, proxy: nil)
      @upstream = upstream
      @proxy = proxy
      @name = name
    end

    def self.all
      request = Net::HTTP::Get.new("/proxies")
      response = http.request(request)
      response.value # raises if not OK

      JSON.parse(response.body).map do |name, attrs|
        Toxiproxy::Proxy.new(
          upstream: attrs["Upstream"],
          proxy: attrs["Listen"],
          name: attrs["Name"]
        )
      end
    end

    def state(new_state, options = {}, &block)
      raise "State #{new_state} is not defined" unless new_state == :down

      destroy
      yield
    ensure
      create
    end

    private
    def self.http
      @http ||= Net::HTTP.new("localhost", 8474)
    end

    def http
      self.class.http
    end

    def create
      request = Net::HTTP::Post.new("/proxies")

      hash = {Upstream: upstream, Name: name}
      hash[:Listen] = @proxy
      request.body = hash.to_json

      response = http.request(request)
      response.value # raises if not OK

      new = JSON.parse(response.body)
      @proxy = new["Listen"]

      self
    end

    def destroy
      request = Net::HTTP::Delete.new("/proxies/#{name}")
      response = http.request(request)
      response.value # raises if not OK

      self
    end
  end

  class << self
    attr_accessor :proxies

    def proxy(name)
      Toxiproxy::Proxy.all.find { |proxy| proxy.name == name.to_s }
    end
    alias_method :[], :proxy
  end
end
