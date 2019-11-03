# VPN Daemon

Simple daemon that provides a TCP socket API protected by TLS as an abstraction 
on top of the management port of (multiple) OpenVPN server process(es). The API
exposes functionality to retrieve a list of connected VPN clients and also 
allows for disconnecting clients.

## Why?

On the VPN server we need to manage multiple OpenVPN processes. Each OpenVPN 
process exposes it management interface through a (TCP) socket. This works fine 
if when the OpenVPN processes and the VPN portal run on the same machine. If 
both the portal and OpenVPN processes run on different hosts this is not 
secure as there is no TLS, and inefficient, i.e. we have to query all OpenVPN 
management ports over the network.

Currently, when using multiple hosts, one MUST have a secure channel between
the nodes, which is something we do not want to require. A simple TLS channel 
over the open Internet should be enough.

This daemon will provide the exact same functionality as the current situation,
except the portal will talk to only one socket, protected using TLS.

## How?

What we want to build is a simple daemon that runs on the same node as the 
OpenVPN processes and is reachable over a TCP socket protected by TLS. The 
daemon will then take care of contacting the OpenVPN processes through their 
local management ports and execute the commands. We want to make this 
"configuration-less", i.e. the daemon should require no additional 
configuration.

Currently there are two commands used over the OpenVPN management connection: 
`status` and `kill` where `status` returns a list of connected clients, and 
`kill` disconnects a client.

In a default installation our VPN server has two OpenVPN processes, so the 
daemon will need to talk to both OpenVPN processes. The portal can just talk to 
the daemon and issues a command there. The results will be merged by the 
daemon. 

Furthermore, we can simplify the API uses to retrieve the list of connected 
clients and disconnect clients. We will only expose what we explicitly use 
and need, nothing more.

## Before

Current situation:

                   .----------------.
                   | Portal         |
          .--------|                |------.
          |        '----------------'      |
          |                                |
          |                                |
          |                                |
          |                                |
          |Local/Remote TCP Socket         |Local/Remote TCP Socket
          |                                |
          v                                v
    .----------------.               .----------------.
    | OpenVPN 1      |               | OpenVPN 2      |
    |                |               |                |
    '----------------'               '----------------'

## After

                  .----------------.
                  | Portal         |
                  |                |
                  '----------------'
                           |
                           | Local/Remote TCP+TLS Socket
                           v
                  .----------------.
                  | Daemon         |
          .-------|                |-------.
          |       '----------------'       |
          |                                |
          |Local TCP Socket                |Local TCP Socket
          |                                |
          v                                v
    .----------------.               .----------------.
    | OpenVPN 1      |               | OpenVPN 2      |
    |                |               |                |
    '----------------'               '----------------'

## Benefits

The daemon will be written in Go, which can handle connections to the OpenVPN
management port concurrently. It doesn't have to do one after the other as is
currently the case. This may improve performance.

We can use TLS with the daemon and require TLS client certificate 
authentication. 

The parsing of the OpenVPN "legacy" protocol and merging of the 
information can be done by the daemon.

We can also begin to envision implementing other VPN protocols when we have
a control daemon, e.g. WireGuard. The daemon would need to have additional 
commands then, i.e. `setup` and `teardown`.

## Daemon API

### Command / Response

Currently 4 commands are implemented:

* `SET_PORTS`
* `DISCONNECT`
* `LIST`
* `QUIT`

The commands are given, optionally with parameters, and the response will be 
of the format:
    
    OK: n

Where `n` is the number of rows the response contains. This is an integer >= 0. 
See the examples below.

If a command is not supported, malformed, or a command fails, the response 
starts with `ERR`, e.g.:

    FOO
    ERR: NOT_SUPPORTED

### Setup

As we want to go for "zero configuration", we want the portal to specify which
OpenVPN management ports we want to talk to.

    SET_PORTS 11940 11941

This works well for single profile VPN servers, but if there are multiple 
profiles involved, one has to specify them all in case of `DISCONNECT`, and 
a subset (just the ones for the profile one is interested in) when calling 
`LIST`.

### Disconnect 

`DISCONNECT` will disconnect the mentioned CN(s).

    DISCONNECT <CN_1> <CN_2> ... <CN_n>

Example:

    DISCONNECT 07d1ccc455a21c2d5ac6068d4af727ca
    
Response:

    OK: 0

### List

This will list all currently connected clients to the configured OpenVPN 
management ports.

    LIST

    ${CN} ${IPv4} ${IPv6}

Example:

    LIST

Response:

    OK: 2
    07d1ccc455a21c2d5ac6068d4af727ca 10.42.42.2 fd00:4242:4242:4242::1000
    9b8acc27bec2d5beb06c78bcd464d042 10.132.193.3 fd0b:7113:df63:d03c::1001

### Quit

    QUIT

## Build & Run

    $ go build -o _bin/vpn-daemon vpn-daemon/main.go

Or use the `Makefile`:

	$ make

## Run

    $ _bin/vpn-daemon

On can then `telnet` to port `41194`, and issue commands:

    $ telnet localhost 41194
    Trying ::1...
    Connected to localhost.
    Escape character is '^]'.
    SET_PORTS 11940 11941
    OK: 0
    DISCONNECT foo
    OK: 0
    QUIT

By default the daemon listens on `localhost:41194`. If you want to modify this
you can specify the `-listen` option to change this, e.g.:

    $ _bin/vpn-daemon -listen 192.168.122.1:41194

### TLS 

We use [vpn-ca](https://git.tuxed.net/LC/vpn-ca) to generate a CA:
    
    $ vpn-ca -init
    $ vpn-ca -server server
    $ vpn-ca -client client

If you want to enable TLS, i.e. require clients to connect over TLS, start 
the daemon with the `-enable-tls` flag, e.g.

    $ _bin/vpn-daemon -enable-tls

If you want to change the path where the CA, certificate and key are located, 
you can recompile the daemon with flags, e.g.:

    $ go build -o _bin/vpn-daemon -ldflags="-X main.tlsCertDir=/path/to/cert -X main.tlsKeyDir=/path/to/key" vpn-daemon/main.go

## Test

    $ make test
