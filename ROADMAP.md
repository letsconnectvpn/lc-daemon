# VPN Daemon

The node(s) themselves _also_ contact the portal when a user connects to the 
VPN. This is currently done by talking to an API over HTTPS. This 
connection is necessary to:

1. decide whether or not the certificate is still allowed to connect (the 
   portal maintains a list of valid CNs per user);
2. decide whether a particular CN (bound to a user) is allowed to connect to a 
   particular VPN profile;
3. register the connection in order to keep a log of connections.
4. retrieve node-configuration files, when deploying a new node
5. retrieve firewall-rules connected to the node

So, this is bi-directional communication between the portal and 
node(s). Ideally we'd get rid of the bidirectional nature of the communication 
and turn into "push" or "pull" where only the portal, or the node(s) initiate
the communication.

Ideally the VPN node(s) could operate for a limited time without being able to
connect to the portal, but keep a local administration of connections instead 
of refusing the client to connect.

## VPN Daemon 2.0

The main goal for v2.0 is to (partially) remove the HTTPS/API communication
going from the node to the portal.

_All the mentioned calls will be done in the TCP-channel._ 
_Responses to these calls will be later defined._

**Firstly**, the certificate and user checking will be completly moved to the DAEMON.
This will remove the need to make an API-call to the portal everytime users 
tries to use the VPN.
To achieve this, the DAEMON will need to have local storage with certificates and
users details. When a new certificate is created/removed or a user no longer has
service-access, the portal will send the new changes to the DAEMON.

These changes will be send in the following format (can be changed in the future):

    SETUP {CN}{SPACE}{[PROFILE_ID,PROFILE_ID]}{SPACE}{EXPIRE_DATE}

In this situation, even if the portal is offline, the node/DAEMON will continue to 
work independent of the portal.


**Secondly**, the portal will no longer be responsible for logging. This will be 
handled by the DAEMON, by the means of `local log-files / remote syslog`.
If the portal is in need of the latest log-information, a simple call to the 
DAEMON will suffice. This can be done when an admin accesses the portal or it can 
be called periodically e.g; everyday at 00:00. 

The call will have the following format:

    GET_LOG

**However**, the API-calls over HTTPS cannot be fully removed.
If a new node is added to an existing portal (or a node stops functioning for an 
extended time), all the existing certificates and users details are not send to 
the DAEMON/node. We can prevent this by sending all certificates and users details 
in the `SETUP` call instead of just the changes, but this will not be ideal.
In this case we opted to use an API-call to the portal to retrieve all the details
when the node/DAEMON is started.
The same counts for retrieving the node-configuration files and firewall-rules