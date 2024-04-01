# RPC Protocol

Pico upstream listeners and Pico servers communicate via bidirectional RPC.
Either peer can send an RPC request to the other.

## Requirements

### WebSockets
One of Pico's design goals is to be easy to host, so it must support being
hosted behind a HTTP(S) load balancer, such as an AWS or GCP application load
balancer, or Kubernetes gateway.

As HTTP is unidirectional (only supporting sending requests from the client to
the server), it isn't suitable for bidirectional RPC. Therefore Pico uses
WebSockets for communication which provides a bidirectional connection between
the two peers. As WebSockets use HTTP for the initial handshake they are
typically supported by HTTP(S) load balancers.

### Multiplexed
Since creating a new connection for every proxied request would add a lot of
overhead, each Pico upstream listener has a single connection to a Pico server.
Therefore multiple RPC requests and responses must be multiplexed over a single
connection, where concurrent requests and responses may be interleaved.

## Protocol
Each RPC message has a type and payload containing arbitrary bytes.

Therefore the RPC stream has Go interface:
```go
type Stream interface {
	// RPC sends the given request to the peer and returns the response.
	RPC(ctx context.Context, rpcType RPCType, m []byte) ([]byte, error)
}
```

Each peer must also register handles for the RPC types it supports. When a
request is received, the corresponding handler will be called. Therefore each
stream has a handler with interface:
```go
// RPCHandler is a RPC request handles that returns a response.
type RPCHandler func(m []byte) []byte

type Handler interface {
	// AddHandler registers the handler for the given RPC type.
	AddHandler(rpcType RPCType, handler RPCHandler)
}
```

Note this means the RPC layer does not application errors. Instead the application layer may include errors in the RPC response.

### Connection
Since Pico is a tunnelling reverse proxy, only the upstream listener can open
the connection. It connects to the server at `/pico/v1/listener/{endpoint ID}`.

Note the server reserves `/pico` for Pico control messages and connections,
all other requests are proxied.

### Header
Each RPC request and response is includes a header containing:
- RPC type (`uint16`)
- Message ID (`uint64`)
- Flags (`uint16`)

The header is fixed size and immediately followed by the optional message
payload. Note the message size isn't included since WebSockets are message
oriented.

#### RPC Type
Contains the application RPC type, such as 'bind', 'unbind', ...

#### Message ID
The message ID uniquely identifies each request/response pair. The request
sender will select a message ID equal to the message ID of the last sent
request plus 1. The receiver then echos back the same message ID in the
response.

#### Flags
Flags contains a bitmap of flags:
- Request/response: Set to 0 if the message is a request, set to 1 if the
message is a response
- Not supported: Set if request RPC type does not have a registered handler on
the receiver. Only used in responses
