# VPN Daemon

## Version 2

The node(s) themselves _also_ contact the portal when a user connects to the 
VPN. This is currently done by talking to an API over HTTPS. This 
connection is necessary to:

1. decide whether or not the certificate is still allowed to connect (the 
   portal maintains a list of valid CNs per user);
2. decide whether a particular CN (bound to a user) is allowed to connect to a 
   particular VPN profile;
3. register the connection in order to keep a log of connections.

So, this is bi-directional communication between the portal and 
node(s). Ideally we'd get rid of the bidirectional nature of the communication 
and turn into "push" or "pull" where only the portal, or the node(s) initiate
communication.

Ideally the VPN node(s) could operate for a limited time without being able to
connect to the portal, but keep a local administration of connections instead 
of refusing the client to connect.

The questions thus becomes: 

1. how can we create a communication channel that is not bi-directional? 
2. how do we make sure we do not refuse service to VPN clients because the 
   communication channel is (temporary) down or the valid certificate CNs are 
   not distributed to the VPN node(s) yet?
3. how do we make sure the portal does (eventually) receive the information 
   to maintain a log of connections?

Some possible approaches to this when the portal contacts the nodes:

1. The Let's Connect! client tells the portal it wants to connect, and at that
   instant the node(s) will be informed of this, same for disconnect, so the
   node can "prepare" for the connection, this would also fit really well in
   the Wireguard scenario.
2. The nodes keep a local log that the portal can pull (periodically) for 
   example starting from a particular sequence number.

Some potential issues:

1. If someone is not using the app, there is no way to inform the portal that 
   a client wants to connect...
2. The portal MUST be authoritative and be "eventually consistent" with the 
   data the node(s) have. We want to avoid the need to deal with "conflicts", 
   e.g. the portal disagrees with the node(s);
3. what to do when the node(s) is/are down? waiting for time-out is not great

There may be many more issues, or possible solutions. This seems like a 
start...
