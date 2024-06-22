# TCP Forwarding

This tutorial shows you how to use Piko to forward TCP traffic to upstream
listeners.

Piko supports proxying TCP traffic, though unlike HTTP it requires using either
[Piko forward](../forward/forward.md) or the
[Go SDK](../sdk/go-sdk.md) to map the desired local TCP port to the target
endpoint (as there's no way to identify the target endpoint using raw TCP).

As an example, this runs an upstream Redis server then connects to it via
Piko.

### Prerequisites

* [Install Piko](./install.md)
* [Install Redis](https://redis.io/docs/latest/operate/oss_and_stack/install/install-redis/)

## Piko Server

Start a [Piko server](../server/server.md) node:
```bash
$ piko server
```

See the server [documentation](../server/server.md) for details.

## Redis Upstream

Start the Redis server:
```bash
$ redis-server
```

This will listen for connections on port `6379` by default.

Next run the [Piko agent](../agent/agent.md) alongside the server to register a
Piko endpoint, `my-redis-endpoint`, and forward incoming TCP connections to
port `6379`:
```bash
$ piko agent tcp my-redis-endpoint 6379
```

To keep the tutorial simple, you can run the Redis server and Piko agent
locally, though using Piko you could run them anywhere, as long as the agent
can open an outbound connection to the Piko server. Such as they could be
running behind a firewall or NAT blocking all incoming traffic.

Instead of using the Piko agent, you could also use the
[Go SDK](../sdk/go-sdk.md):
```go
var opts []piko.Option
// ...

client := piko.New(opts...)
if err := client.ListenAndForward(
    context.Background(),
    "my-redis-endpoint",
    "localhost:6379",
);  err != nil {
    panic("forward: " + err.Error())
}
```

## Connect

Finally you can open a TCP connection to the endpoint `my-redis-endpoint` using
[Piko forward](../forward/forward.md).

`piko forward` listens on a local TCP port and forwards connections to the
configured endpoint. Run `piko forward` to listen on port `7000` and forward to
endpoint `my-redis-endpoint`:
```bash
$ piko forward tcp 7000 my-redis-endpoint
```

You can then connect to port `7000` and your connection will be forwarded to
your registered upstream listener:
```bash
$ redis-cli -p 7000 PING
PONG
```

Alternatively you can connect to Redis directly from your application using
the Go SDK. The Piko client has a
`Dial(ctx context.Context, endpointID string) (net.Conn, error)` method
that returns a `net.Conn`. Such as connecting with the
[Redis Go](https://github.com/redis/go-redis) client:
```go
var opts []piko.Option
// ...

client := piko.New(opts...)

// Use a custom dialer that connects to the upstream endpoint via Piko.
dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
    return client.Dial(ctx, addr)
}
// github.com/redis/go-redis/v9
rdb := redis.NewClient(&redis.Options{
    Addr:   "my-redis-endpoint",
    Dialer: dialer,
})

if err := rdb.Ping(context.Background()).Err(); err != nil {
    panic("ping: " + err.Error())
}
```
