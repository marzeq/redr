# redr - (r)ecieve, (e)xecute, (d)isplay, (r)epeat

redr is a simple (and very wip) command runner that can be communicated with via tcp sockets

## installation/building

### from releases

not yet

### from source

[just](https://github.com/casey/just) is used here, so you need to have it installed

```bash
just build # outputs to build/current-target
# or
just build-all # outputs to build/{windows,linux,macos}/{amd64,arm64}
```

## usage

```bash
redr # ctrl+c to stop server
```

## socket api

this is the control flow of the socket api. still wip, expect changes

```
client connects to server
client sends: { "type": "introduce", "cwd"?: "...", "run_next_after_failure"?: true|false } (run_next_after_failure defaults to false, cwd defaults to the server's cwd)
server sends: { "type": "ok" }
client sends: { "type": "run_commands", "commands": ["..."] }

loop:
  server sends: { "type": "command_ran", "exit_code": number }
  client sends: { "type": "ok" } -- just meaning we acknowledged the message, and are ready for the next one, deciding to continue is up to the server

server sends: { "type": "ok" } -- meaning command running process is done, regardless of the exit code
client sends message: { "type": "bye" } -- client communicates that it's done
server sends json: { "type": "ok" } -- server acknowledges the client's message
parties can now close the connection
```

if a client is already connected to the server, the server will immediately respond with a { "type": "kick_off" } message
and will close the connection. the client is expected to handle this and not connect again until the server is has no clients connected

if at any point the server gets an unexpected error, it will also send a { "type": "kick_off" } message and close the connection.
the client is expected to handle this, and shouldn't reonnect until the server is restarted
