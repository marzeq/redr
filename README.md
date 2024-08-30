# redr - (r)un, (e)xecute, (d)isplay, (r)epeat

redr is a simple (and very wip) command runner that can be communicated with via tcp sockets

## usage

```bash
redr # ctrl+c to stop server
```

## socket api

this is the control flow of the socket api

```
client connects to server
client sends message: { "type": "introduce", "cwd"?: "..." }
server sends json: { "type": "ok" }
loop:
  client sends message: { "type": "run_command", "command": "..." }
  server sends json: { "type": "command_ran", "exit_code": 0 }
client sends message: { "type": "bye" }
server sends json: { "type": "ok" }
```

if a client is already connected to the server, the server will immediately respond with a { "type": "kick_off" } message
and will close the connection. the client is expected to handle this and not connect again until the server is has no clients connected

if at any point the server gets an unexpected error, it will also send a { "type": "kick_off" } message and close the connection
the client is expected to handle this, and shouldn't command-runneronnect until the server is restarted
