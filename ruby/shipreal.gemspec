# frozen_string_literal: true

require_relative "lib/shipreal/version"

Gem::Specification.new do |spec|
  spec.name = "shipreal"
  spec.version = ShipReal::VERSION
  spec.authors = ["Michael Lugassy"]
  spec.email = ["michael@shipreal.dev"]

  spec.summary = "SDK and CLI for the public ShipReal course API"
  spec.description = "Search the curriculum, read a module, read pricing. " \
                     "No authentication: the API is public read-only reference " \
                     "data about one course. Standard library only, no runtime " \
                     "dependencies."
  spec.license = "MIT"
  spec.required_ruby_version = ">= 2.6.0"

  # These are how a scanner confirms the gem is the official SDK for the domain
  # rather than a third party with a similar name: RubyGems exposes them as
  # homepage_uri and source_code_uri on the package page and in its API. Go
  # modules have no equivalent field, which is why the Go client has to prove
  # ownership through its module path instead. Keep homepage_uri pointing at
  # shipreal.dev.
  spec.homepage = "https://shipreal.dev/developers"
  spec.metadata = {
    "homepage_uri" => "https://shipreal.dev/developers",
    "documentation_uri" => "https://shipreal.dev/developers",
    "source_code_uri" => "https://github.com/mluggy/shipreal-dev",
    "bug_tracker_uri" => "https://github.com/mluggy/shipreal-dev/issues",
    "changelog_uri" => "https://github.com/mluggy/shipreal-dev/releases",
    "rubygems_mfa_required" => "true"
  }

  spec.files = Dir["lib/**/*.rb", "exe/*", "README.md", "LICENSE"]
  spec.bindir = "exe"
  spec.executables = ["shipreal"]
  spec.require_paths = ["lib"]
end
