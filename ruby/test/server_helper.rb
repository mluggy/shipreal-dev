# frozen_string_literal: true

require "socket"
require "uri"

# A minimal HTTP server on a thread, so the tests exercise the client's real
# socket path without depending on webrick, which left the standard library in
# Ruby 3.0. The suite has to run on the oldest Ruby the gemspec claims (2.6)
# and on a current one, and a test dependency that only exists on one of them
# would quietly mean it is only ever run on the other.
module TestServer
  # One parsed request, with the pieces a test wants to assert on.
  Request = Struct.new(:method, :path, :query, :headers, :body)

  # Yields a base URL. The block's handler receives a Request and returns
  # [status, headers, body].
  def with_server(handler)
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    thread = Thread.new do
      loop do
        begin
          socket = server.accept
        rescue IOError, Errno::EBADF
          break
        end
        begin
          serve(socket, handler)
        rescue StandardError
          nil
        ensure
          socket.close unless socket.closed?
        end
      end
    end
    yield "http://127.0.0.1:#{port}"
  ensure
    server&.close
    thread&.kill
  end

  private

  def serve(socket, handler)
    line = socket.gets
    return unless line

    method, target, = line.split
    headers = {}
    while (h = socket.gets) && h != "\r\n" && h != "\n"
      k, v = h.split(":", 2)
      headers[k.strip.downcase] = v.to_s.strip
    end
    body = nil
    if (len = headers["content-length"])
      body = socket.read(len.to_i)
    end

    uri = URI("http://x#{target}")
    query = URI.decode_www_form(uri.query || "").to_h
    req = Request.new(method, uri.path, query, headers, body)

    status, res_headers, res_body = handler.call(req)
    res_body = res_body.to_s
    socket.write("HTTP/1.1 #{status} #{status == 200 ? 'OK' : 'Error'}\r\n")
    res_headers.each { |k, v| socket.write("#{k}: #{v}\r\n") }
    socket.write("content-length: #{res_body.bytesize}\r\n\r\n")
    socket.write(res_body)
  end
end
