# redr - (r)ecieve, (e)xecute, (d)isplay, (r)epeat

redr is a simple command runner that can be communicated with via tcp sockets

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

```
server supports multiple clients connected at once, but when a new "run" command is sent, the current
command set is cleared and if any command is running it is killed

control flow:

client connects to server
at some point, client sends: { "type": "run", "commands": ["..."], "cwd"?: "...", "run_next_after_failure"?: true|false } (run_next_after_failure defaults to false, cwd defaults to the server's cwd)
server sends to the client: { "type": "ok" }

loop:
  server sends to every client: { "type": "command_ran", "exit_code": number, "cmd": string, "last": boolean } after each command ran

the client can now disconnect

at any point, the client can send a { "type": "ignore" } message, and the server will simply ignore it
```
