require "toxiproxy/version"
require "json"

module Toxiproxy
  class Proxy
    attr_reader :upstream, :proxy, :name

    def initialize(upstream:, proxy: nil, name:)
      @upstream = upstream
      @proxy = proxy
      @name = name
    end

    def self.create(*args)
      self.new(*args).create
    end

    def self.all
      request = Net::HTTP::Get.new("/proxies")
      response = http.request(request)
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
      create
    end

    def create
      request = Net::HTTP::Post.new("/proxies")
      request.body = {Upstream: upstream, Name: name}.to_json
      response = http.request(request)

      new = JSON.parse(response.body)
      @proxy = new["Listen"]

      self
    end

    def destroy
      request = Net::HTTP::Delete.new("/proxies/#{name}")
      http.request(request)
    end

    private
    def self.http
      @http ||= Net::HTTP.new("localhost", 8474)
    end

    def http
      self.class.http
    end
  end

  class << self
    attr_accessor :proxies

    def service(name)
      Toxiproxy::Proxy.all.find { |proxy| proxy.name == name }
    end
    alias_method :[], :service
  end
end
