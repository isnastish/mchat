# Multi-client chat in Go.

## Design document
# Connecting new clients
When a session receives a new connection it runs handleConnection(conn net.Conn) on a separate
go routine. Once we enter this function, we create an instance of a Client and send it on a pendingClientsCh channel.
Once received, we append the client into pendingClients map, so we can sent messages to that client using its connection (net.Conn). When the authentication process is done, we append the information about client into a backend storage (if it was a new joiner), then create a new client instance, set it's name (not a password, should only be stored in a storage using sha256) and insert it to clientsMap sending a message to all other participants that a new client joined as well as sending this client into onSessionJoinCh where it will be appended into a clientMap (or we can insert it inside handleConnection directly). Pending client should be removed from pending clients map.
When the client disconnects, we should update its status to `disconnected`, so we don't send any messages to 
disconnected clients. If it joins again, all the credentials (name and password) has to be verified against
the local backing storage, and clientsMap has to be updated with a new IpAddr and a new connection instance `net.Conn`.

We can maintain client's state `ClientState_Pending`, `ClientState_Connected` and `ClientState_Disconnected`
which would be a part of a Client struct. When the client has a `Pending` state, its ip address is used as a key in a s.clients map. Once it has been authenticated or registered, we have to delete use the name as a key instead of ip address. That would require to delete an element from a hash map and inserte it with a new key updating the status to `Connected`. If the client disconnects, we should change the status to
`Disconnected`. If he tries to rejoin later, (and this is the problem, because how do we match it)

# Messages

# Credentials
Password is hashed using sha256 algorithm and put into a backing storage. The next time client tries to rejoin, we parse its password, compute the hash and match against the one in a database. If the password matches, we connect the client and give it full access to a session, if not, we ask to enter a password once again (we might introduce
a retry limit here).

```Go
type Connection struct {
    conn net.Conn 
    ipAddr string
}

type Client struct {
    conn Connection
}
```
