require 'socket'

server = TCPServer.new(8000)

loop do
  puts "Waiting for client.."
  server.accept
  puts "Got client.."

  p server.read
end
