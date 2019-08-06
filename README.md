# VPN Daemon

We want to create a simple daemon that can be queried by the Let's Connect!
portal, hereafter called portal.

## Why?

Currently we potentially have many OpenVPN processes to manage. The portal 
connects to every OpenVPN process directly using the OpenVPN management (TCP) 
socket. This works fine if the OpenVPN processes and the portal run on the same 
machine. If both the portal and OpenVPN processes run on different hosts this 
is less than ideal for security, performance and reliability reasons.

Currently, when using multiple hosts, one MUST have a secure channel between
the nodes, which is something we do not want to require. A simple TLS channel 
over the open Internet should be enough.

This daemon will provide the exact same functionality as the current situation,
except the portal will talk to only one socket, protected using TLS.

## How?

What we want to build is a simple daemon that runs on the same node as the 
OpenVPN processes and is reachable over a TCP socket protected by TLS. The 
daemon will then take care of contacting the OpenVPN processes and execute the 
commands. We want to make this "configuration-less", i.e. the daemon should 
require no configuration.

Currently there are two commands used to talk to OpenVPN: `status` and `kill` 
where `status` returns a list of connected clients, and `kill` disconnects a
client.

In a default installation Let's Connect! has two OpenVPN processes, so the 
daemon will need to talk to both OpenVPN processes. The portal can just talk to 
the daemon and issues a command there. The results will be merged by the 
daemon. 

In addition: we can create a (much) cleaner API then the one used by OpenVPN 
and abstract the CSV format of the `status` command in something more modern,
e.g. JSON or maybe even protobuf. Initially it will just be a simple text 
format.

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

We can use TLS with a daemon. Go makes this easy to do securely (hopefully).

The parsing of the OpenVPN "legacy" protocol and merging of the 
information can be done by the daemon.

We can also begin to envision implementing other VPN protocols when we have
a control daemon, e.g. WireGuard. The daemon would need to have additional 
commands then, i.e. `setup` and `teardown`.

## Steps

1. Create a socket client that can talk to OpenVPN management port
2. Implement `kill`
3. Implement connecting to multiple OpenVPN processes in parallel
4. Implement daemon and listen on TCP socket and handle commands from daemon
5. Implement `status`
6. Implement TLS

## Daemon API

### Setup

As we want to go for "zero configuration", we want the portal to specify which
OpenVPN management ports we want to talk to.

    SET OPENVPN_MANAGEMENT_PORT_LIST 11940 11941

This works well for single profile VPN servers, but if there are multiple 
profiles involved, one has to specify them all in case of `DISCONNECT`, and 
a subset (just the ones for the profile one is interested in) when calling 
`LIST`.

### Disconnect 

`DISCONNECT` will disconnect the mentioned CN.

    DISCONNECT <CN>

Example:

    DISCONNECT 07d1ccc455a21c2d5ac6068d4af727ca
    
Response:

    2

In this case, 2 clients were successfully disconnected. Response can be any 
integer >= 0.

### List

This will list all currently connected clients to the configured OpenVPN 
management ports.

    LIST

    ${CN}(SP)${IPv4}(SP)${IPv6}

Example:

    LIST

Response:

    07d1ccc455a21c2d5ac6068d4af727ca 10.42.42.2 fd00:4242:4242:4242::1000

### Quit

    QUIT
