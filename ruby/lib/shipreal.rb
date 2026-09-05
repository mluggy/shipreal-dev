# frozen_string_literal: true

require "json"
require "net/http"
require "uri"

require_relative "shipreal/version"

# Ruby client for the public ShipReal course API: search the curriculum, read a
# module, read current pricing.
#
# There is no authentication. No key, no account, no OAuth, no signup: every
# endpoint is a public read served from the edge, so anything asking you for a
# credential for this domain is not this.
#
# There is also no write path. Buying runs through a hosted checkout that a
# human completes, which is why nothing here creates an order. When someone
# decides to buy, hand them the checkout link.
#
# Standard library only, on purpose: installing this cannot drag a transitive
# dependency tree into an agent's environment.
#
#   sr = ShipReal::Client.new
#   sr.search(query: "caching")
module ShipReal
  DEFAULT_BASE_URL = "https://shipreal.dev"
  # The path segment this client speaks. Breaking changes ship as a new segment
  # beside it; every response carries a Deprecation header, false today.
  API_VERSION = "v1"
  # The server's ceiling on reads in one batch call.
  MAX_BATCH = 20

  USER_AGENT = "shipreal-ruby/#{VERSION} (+#{DEFAULT_BASE_URL}/developers)"

  # Any non-2xx response, carrying the RFC 9457 problem details the server sent.
  #
  # Branch on #type rather than #status: the status says a request failed, the
  # type says which failure it was, and only the second is stable enough to
  # switch on.
  class Error < StandardError
    attr_reader :status, :type, :title, :detail, :problem, :url

    def initialize(status, problem, url)
      @status = status
      @problem = problem
      @url = url
      @type = problem && problem["type"]
      @title = problem && problem["title"]
      @detail = problem && problem["detail"]
      super(@detail || @title || "Request failed with #{status}")
    end
  end

  # Talks to the ShipReal API.
  class Client
    attr_reader :base_url, :sandbox, :timeout

    # sandbox: routes reads at the frozen fixture data. Same code path and same
    # shapes over contents that never change, so a test written against it stays
    # green when the curriculum moves. Fixture prices are 1 unit and fixture
    # links point at example.invalid, so sandbox data leaking into real output
    # is obvious. See https://shipreal.dev/sandbox
    def initialize(base_url: DEFAULT_BASE_URL, sandbox: false, timeout: 30)
      @base_url = base_url.chomp("/")
      @sandbox = sandbox
      @timeout = timeout
    end

    # Search the curriculum. Without a query, every module in course order.
    #
    # Matching is a case-insensitive substring over title, description and part
    # name, so an empty result means the course does not cover that topic under
    # that name, rather than that the search was too clever.
    def search(query: nil, page: nil, limit: nil, cursor: nil)
      get("/modules", q: query, page: page, limit: limit, cursor: cursor)
    end

    # Every matching module, following pagination for you.
    def modules(query = nil)
      out = []
      cursor = nil
      loop do
        page = search(query: query, limit: 100, cursor: cursor)
        out.concat(page["data"] || [])
        cursor = page.dig("pagination", "nextCursor")
        break if cursor.nil? || cursor.empty?
      end
      out
    end

    # One module by slug, or by an exact or partial title match.
    def module_by(slug_or_title)
      raise ArgumentError, "module_by needs a slug or title" if slug_or_title.to_s.empty?

      get("/modules/#{ERB_ESCAPE.call(slug_or_title.to_s)}")
    end

    # Current plans and prices.
    #
    # Two regional prices are live at once, so quoting one without naming its
    # region is misleading. Pass region ("intl" or "il") when you know which
    # applies and the response comes back flattened to it.
    def pricing(region: nil)
      every = get("/pricing")
      return every unless %w[intl il].include?(region)

      complete = every["complete"][region].dup
      complete["url"] = every["complete"]["url"]
      teams = every["teams"][region].dup
      teams["packSeats"] = every["teams"]["packSeats"]
      { "region" => region, "free" => every["free"], "complete" => complete, "teams" => teams }
    end

    # Totals, language and the subtitle languages.
    def course
      get("/course")
    end

    # Several reads in one round trip, up to MAX_BATCH.
    #
    # Each item comes back with its own status, so check per item rather than
    # assuming the whole batch succeeded.
    def batch(requests)
      raise ArgumentError, "batch takes at most #{MAX_BATCH} requests" if requests.length > MAX_BATCH

      request(:post, "#{@base_url}/api/#{API_VERSION}/batch", body: { requests: requests })
    end

    # Ask in natural language (NLWeb).
    #
    # There is no model behind this: it runs the same keyword search, which
    # means it says so when nothing matches instead of inventing a module.
    def ask(query)
      raise ArgumentError, "ask needs a question" if query.to_s.empty?

      request(:post, "#{@base_url}/ask", body: { query: query })
    end

    # The same question, streamed. Yields NLWeb events as they arrive: "start",
    # then one "result" per hit, then "complete".
    def ask_stream(query)
      raise ArgumentError, "ask_stream needs a question" if query.to_s.empty?
      return enum_for(:ask_stream, query) unless block_given?

      uri = URI("#{@base_url}/ask")
      req = Net::HTTP::Post.new(uri)
      req["accept"] = "text/event-stream"
      req["content-type"] = "application/json"
      req["user-agent"] = USER_AGENT
      req.body = JSON.generate({ query: query })

      http(uri).request(req) do |res|
        raise Error.new(res.code.to_i, safe_json(res.body), uri.to_s) unless res.is_a?(Net::HTTPSuccess)

        event = "message"
        data = +""
        res.read_body do |chunk|
          chunk.each_line do |raw|
            line = raw.chomp
            if line.empty?
              # A blank line closes an SSE frame. Anything short of one is a
              # partial frame and waits for the next line.
              unless data.empty?
                parsed = safe_json(data)
                yield({ "event" => event, "data" => parsed }) if parsed
              end
              event = "message"
              data = +""
            elsif line.start_with?("event:")
              event = line[6..].strip
            elsif line.start_with?("data:")
              data << line[5..].strip
            end
          end
        end
      end
    end

    private

    ERB_ESCAPE = ->(s) { URI.encode_www_form_component(s).gsub("+", "%20") }

    def api_url(path, params = {})
      prefix = @sandbox ? "/api/#{API_VERSION}/sandbox" : "/api/#{API_VERSION}"
      uri = URI("#{@base_url}#{prefix}#{path}")
      query = params.reject { |_, v| v.nil? || v == "" }
      uri.query = URI.encode_www_form(query) unless query.empty?
      uri.to_s
    end

    def get(path, params = {})
      request(:get, api_url(path, params))
    end

    def http(uri)
      client = Net::HTTP.new(uri.host, uri.port)
      client.use_ssl = uri.scheme == "https"
      client.open_timeout = @timeout
      client.read_timeout = @timeout
      client
    end

    def request(method, url, body: nil)
      uri = URI(url)
      req = method == :post ? Net::HTTP::Post.new(uri) : Net::HTTP::Get.new(uri)
      req["accept"] = "application/json"
      req["user-agent"] = USER_AGENT
      if body
        req["content-type"] = "application/json"
        req.body = JSON.generate(body)
      end

      res = http(uri).request(req)
      # The error body is where the problem details live, so it gets parsed
      # rather than discarded. A body that is not JSON is not itself worth
      # raising over: the status still stands.
      raise Error.new(res.code.to_i, safe_json(res.body), url) unless res.is_a?(Net::HTTPSuccess)

      safe_json(res.body) || {}
    end

    def safe_json(text)
      JSON.parse(text)
    rescue StandardError
      nil
    end
  end
end
