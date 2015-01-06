class Toxiproxy < Formula
  homepage "https://github.com/Shopify/toxiproxy"
  url "http://shopify-vagrant.s3.amazonaws.com/toxiproxy/1.0.0-darwin-amd64"
  sha1 "05de584c56572330e6dd5c1cb2d38a5f388fe76e"
  version "1.0.0rc1"

  depends_on :arch => :x86_64

  def install
    bin.install "1.0.0-darwin-amd64" => "toxiproxy"
  end
end
