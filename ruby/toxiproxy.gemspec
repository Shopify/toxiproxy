# coding: utf-8
lib = File.expand_path('../lib', __FILE__)
$LOAD_PATH.unshift(lib) unless $LOAD_PATH.include?(lib)
require 'toxiproxy/version'

Gem::Specification.new do |spec|
  spec.name          = "toxiproxy"
  spec.version       = Toxiproxy::VERSION
  spec.authors       = ["Simon Eskildsen"]
  spec.email         = ["sirup@sirupsen.com"]
  spec.summary       = %q{Library to test for network resiliency}
  spec.description   = %q{Toxiproxy is a library to test for network resiliency in your Ruby apps}
  spec.homepage      = ""
  spec.license       = "MIT"

  spec.files         = `git ls-files`.split($/)
  spec.executables   = spec.files.grep(%r{^bin/}) { |f| File.basename(f) }
  spec.test_files    = spec.files.grep(%r{^(test|spec|features)/})
  spec.require_paths = ["lib"]

  spec.add_development_dependency "bundler"
  spec.add_development_dependency "rake"
end
