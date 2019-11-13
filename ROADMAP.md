# Roadmap

VPN Daemon 1 took care of moving the traffic between controller and node(s) to 
a TLS protected connection.

However, the node(s) still need to contact the controller in a number of 
situation:

* Decide whether a client attempting to connect is allowed to (still) connect
  (to a particular profile);
* Make note of the connection in the log (connect time, disconnect time);
* Obtain the OpenVPN configuration for the node (setup);
* Obtain the firewall configuration for the node (setup);

The first two are done for every connection. The last two are done only when 
the admin attempts to reconfigure the VPN. 

The goal of VPN Daemon 2 is to get rid of the first two connections, i.e. 
during "normal" operation, there is no (direct) connection from the node(s) the 
controller. At least this allows node(s) to operate independently from the 
controller for some time.

The goal of VPN Daemon 3 is to get rid of the last two as well.

# VPN Daemon 2

This requires two things in the node:

1. A list of certificate CNs and their allowed profile(s), with an expiry time;
2. A way to write to a log file (on connect, on disconnect);

In order for this to be useful and cover the current use cases, we need to 
introduce extra API calls the controller can call on the daemon:

1. The controller should be able to inform the node(s) about new CNs, new 
   profiles and new expiry times;
2. The controller should be able to "disable" the CNs of a particular user, or
   update the list of allowed profiles;
3. The controller should be able to query the node(s) for their log based on 
   IP address and time stamp to be able to handle abuse complaints
4. There should be some mechanism for a node to retrieve all CNs when setting
   up a new node, or when the node loses its state.

## Certificate CN List

In order for the controller to inform the node(s) of new certificates, a call
`SETUP` is introduced. It takes the CN, list of allowed profiles, and an 
"expiry date", so the node can clean up the list periodically to avoid the list
to grow indefinitely.

    SETUP ${CN} [${PROFILE_1},${PROFILE_2},${PROFILE_n}] ${EXPIRES_AT}

Example:

    SETUP 9b8acc27bec2d5beb06c78bcd464d042 [internet,internet-utrecht] 2019-01-01T08:00:00+00:00

Example response:

    OK: 0

The node will need to store this information, for example in the local file 
system:

    /var/lib/vpn-daemon/c/9b8acc27bec2d5beb06c78bcd464d042/internet
    /var/lib/vpn-daemon/c/9b8acc27bec2d5beb06c78bcd464d042/internet-utrecht

The files themselves contain the expiry time.

Disabling a user can be done using the following call:

    SETUP 9b8acc27bec2d5beb06c78bcd464d042 [] 2019-01-01T08:00:00+00:00

This would then remove the directory as there are no profiles the user needs to 
have access to anymore.

### Sync

When the node loses its state, there needs to be a call that can be used to 
populate the CN directory. 

We can for example tie this in the `vpn-server-node-server-config` script in 
order to always make sure we are in "sync" when setting up the node(s).

## Log

In order to obtain the log from a node, the `LOG` call is introduced:

    LOG 10.42.42.42 2019-01-01T08:00:00+00:00

This would return (if there is a log entry) which CN was connected to the VPN 
using this IP at the provided time.

Example response:

    OK: 1
    9b8acc27bec2d5beb06c78bcd464d042

For some setups it may make sense to also setup "remote syslog" on the node(s) 
to log to a central log server. This is already possible.

### Questions

- How to store the log on the file system in a reasonable way?

# Optimization

Not all nodes are responsible for all profiles, so it would be nice to only 
send the `SETUP` calls to the relevant nodes. The same is true for the `LOG` 
call: every node has a different IP range to deal with, so we only need to 
send the `LOG` call to the node that has this IP range configured instead of 
all of them.
