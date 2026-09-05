# frozen_string_literal: true

require "minitest/autorun"
require "json"
require_relative "server_helper"
require_relative "../lib/shipreal"

# A real socket rather than a stub, so the client's own HTTP handling (headers,
# query building, SSE framing, error parsing) is what gets exercised.
class ShipRealTest < Minitest::Test
  include TestServer

  MODULE_ONE = {
    "slug" => "sandbox-module-1", "title" => "Sandbox: Thinking in Systems & State",
    "part" => "Sandbox Part 1", "description" => "A fixed fixture module.",
    "chapters" => 12, "minutes" => 29, "url" => "https://example.invalid/sandbox-module-1"
  }.freeze

  JSON_HEADERS = { "content-type" => "application/json" }.freeze

  def json_ok(body)
    [200, JSON_HEADERS, JSON.generate(body)]
  end

  # Runs the block with a client pointed at a server driven by `handler`.
  def serving(handler)
    with_server(handler) do |base|
      yield ShipReal::Client.new(base_url: base)
    end
  end

  def test_search_sends_query_and_parses_page
    seen = nil
    serving(lambda { |req|
      seen = req
      json_ok({ "data" => [MODULE_ONE] })
    }) do |client|
      page = client.search(query: "caching", limit: 50)
      assert_equal 1, page["data"].length
      assert_equal MODULE_ONE["slug"], page["data"][0]["slug"]
    end
    assert_equal "/api/v1/modules", seen.path
    assert_equal "caching", seen.query["q"]
    assert_equal "50", seen.query["limit"]
    assert_match(/\Ashipreal-ruby\//, seen.headers["user-agent"])
  end

  def test_sandbox_routes_under_sandbox_prefix
    seen = nil
    with_server(lambda { |req|
      seen = req
      json_ok({ "data" => [] })
    }) do |base|
      ShipReal::Client.new(base_url: base, sandbox: true).search
    end
    assert_equal "/api/v1/sandbox/modules", seen.path
  end

  def test_modules_follows_cursor_then_stops
    calls = 0
    serving(lambda { |req|
      calls += 1
      if req.query["cursor"].nil?
        json_ok({ "data" => [MODULE_ONE],
                  "pagination" => { "hasMore" => true, "nextCursor" => "page2" } })
      else
        json_ok({ "data" => [{ "slug" => "second", "cursorSeen" => req.query["cursor"] }],
                  "pagination" => { "hasMore" => false, "nextCursor" => nil } })
      end
    }) do |client|
      all = client.modules
      assert_equal 2, all.length
      assert_equal "second", all[1]["slug"]
      assert_equal "page2", all[1]["cursorSeen"]
    end
    assert_equal 2, calls
  end

  # An empty-string cursor has to terminate the loop as surely as a nil one, or
  # a server that sends one spins the client forever.
  def test_modules_stops_on_empty_cursor
    calls = 0
    serving(lambda { |_req|
      calls += 1
      json_ok({ "data" => [MODULE_ONE],
                "pagination" => { "hasMore" => true, "nextCursor" => "" } })
    }) { |client| client.modules }
    assert_equal 1, calls
  end

  def test_module_escapes_the_path_segment
    seen = nil
    serving(lambda { |req|
      seen = req
      json_ok(MODULE_ONE)
    }) { |client| client.module_by("systems & state") }
    # Both the space and the ampersand are encoded. An unescaped & in a path
    # segment is the kind of thing that works until a title contains one.
    assert_equal "/api/v1/modules/systems%20%26%20state", seen.path
  end

  def test_problem_details_become_an_error
    serving(lambda { |_req|
      [404, { "content-type" => "application/problem+json" }, JSON.generate({
        "type" => "https://shipreal.dev/developers#errors-module-not-found",
        "title" => "Module Not Found", "status" => 404,
        "detail" => 'No module matches "nope".'
      })]
    }) do |client|
      err = assert_raises(ShipReal::Error) { client.module_by("nope") }
      assert_equal 404, err.status
      assert_match(/#errors-module-not-found\z/, err.type)
      assert_match(/No module matches/, err.message)
    end
  end

  # A non-JSON error body must not swallow the status, which is the only thing
  # the caller can still act on.
  def test_non_json_error_body_keeps_status
    serving(->(_req) { [502, {}, "upstream fell over"] }) do |client|
      err = assert_raises(ShipReal::Error) { client.course }
      assert_equal 502, err.status
      assert_nil err.type
    end
  end

  def test_pricing_region_flattens_deliberately
    body = {
      "free" => { "price" => 0, "includes" => "overviews" },
      "complete" => { "intl" => { "now" => "$99" }, "il" => { "now" => "₪299" }, "url" => "u" },
      "teams" => { "intl" => { "now" => "$792" }, "il" => { "now" => "₪2,392" }, "packSeats" => 10 }
    }
    serving(->(_req) { json_ok(body) }) do |client|
      il = client.pricing(region: "il")
      assert_equal "il", il["region"]
      assert_equal "₪299", il["complete"]["now"]
      assert_equal 10, il["teams"]["packSeats"]
      # No region named returns both, unflattened, rather than guessing one.
      both = client.pricing
      assert both["complete"].key?("intl")
      assert both["complete"].key?("il")
    end
  end

  def test_batch_refuses_over_the_ceiling
    client = ShipReal::Client.new
    assert_raises(ArgumentError) do
      client.batch(Array.new(ShipReal::MAX_BATCH + 1) { { "path" => "/pricing" } })
    end
  end

  def test_batch_posts_requests
    seen = nil
    serving(lambda { |req|
      seen = req
      json_ok({ "count" => 1, "responses" => [{ "id" => "0", "status" => 200 }] })
    }) do |client|
      out = client.batch([{ "path" => "/pricing" }])
      assert_equal 1, out["count"]
    end
    assert_equal "POST", seen.method
    assert_equal "/api/v1/batch", seen.path
    assert_equal [{ "path" => "/pricing" }], JSON.parse(seen.body)["requests"]
  end

  def test_ask_stream_yields_events_in_order
    seen = nil
    serving(lambda { |req|
      seen = req
      [200, { "content-type" => "text/event-stream" },
       "event: start\ndata: {\"_meta\":{\"count\":1}}\n\n" \
       "event: result\ndata: {\"results\":[{\"name\":\"Caching\"}]}\n\n" \
       "event: complete\ndata: {\"_meta\":{}}\n\n"]
    }) do |client|
      events = []
      names = []
      client.ask_stream("caching") do |ev|
        events << ev["event"]
        Array(ev.dig("data", "results")).each { |r| names << r["name"] }
      end
      assert_equal %w[start result complete], events
      assert_equal ["Caching"], names
    end
    assert_equal "text/event-stream", seen.headers["accept"]
  end

  def test_empty_arguments_are_rejected_before_any_request
    # Points at a closed port: reaching the network at all would be the failure.
    client = ShipReal::Client.new(base_url: "http://127.0.0.1:1")
    assert_raises(ArgumentError) { client.module_by("") }
    assert_raises(ArgumentError) { client.ask("") }
    assert_raises(ArgumentError) { client.ask_stream("") { nil } }
  end
end
