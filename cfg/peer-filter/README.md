# Peer Filter

The peer filter allow to filter who can hop on your node.

To use it first create an PeerFilter object:

You may add some peer id allowed to hop here, that way faster than doing it
after the object creation.
```go
pf := filter.New(peer1, peer2)
```
You can then create your relay and apply the acceptor.
```go
r, _ := relay.NewRelay(ctx, host, upgrader, relay.OptHop, relay.OptApplyAcceptor(pf))
```
That will filter who can hop.

You may now add or remove people allowed to hop.
```go
pf.Allow(peer3)
pf.Unallow(peer3)
```
