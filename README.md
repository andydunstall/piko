<p align="center">
  <img src="assets/images/logo.png?raw=true" width='40%'>
</p>

---

- [What Is Piko?](#what-is-piko)
- [Design Goals](#design-goals)
- [Getting Started](#getting-started)
- [How Piko Works](#how-piko-works)
- [Support](#support)
- [Docs](#docs)
- [Contributing](#contributing)
- [License](#license)

## What Is Piko?

Piko is a reverse proxy that provides a secure way to connect to services that
aren’t publicly routable, known as tunneling. Instead of sending traffic
directly to your services, your upstream services open outbound-only
connections (tunnels) to Piko, then Piko forwards traffic to your services via
their established connections.

Piko has two key design goals:
* Built to serve production traffic by running as a cluster of nodes fault
tolerance, horizontal scaling and zero-downtime deployments
* Simple to host behind a HTTP(S) load balancer on Kubernetes

Therefore Piko can be used as an open-source alternative to
[Ngrok](https://ngrok.com/).

Such as you may use Piko to expose services in a customer network, a bring your
own cloud (BYOC) service, or to connect to user devices.

## Features

### Reverse Proxy

In a traditional reverse proxy, you configure routing rules describing how to
route incoming traffic to your upstream services. The proxy will then open
connections to your services and forward incoming traffic. This means your
upstream services must be discoverable and have an exposed port that's
accessible from the proxy.

Whereas with Piko, your upstreams open outbound-only connections to the
[Piko server](./docs/server/server.md) and specify what endpoint they are
listening on. Piko then forwards incoming traffic to the correct upstream via
its outbound connection.

Therefore your services may run anywhere without requiring a public route, as
long as they can open a connection to the Piko server.

### Endpoints

Upstream services listen for traffic on a particular endpoint. Piko then
manages routing incoming connections and requests to an upstream service
listening on the target endpoint. If multiple upstreams are listening on the
same endpoint, requests are load balanced among the available upstreams.

No static configuration is required to configure endpoints, upstreams can
listen on any endpoint they choose.

Such as to open a HTTP or TCP upstream listener using the
[Piko agent](./docs/agent/agent.md) that listens on endpoint `my-endpoint` and
forwards traffic to `localhost:3000`:
```
# HTTP listener.
$ piko agent http my-endpoint 3000

# TCP listener.
$ piko agent tcp my-endpoint 3000
```

You can also use the [Go SDK](./docs/sdk/go-sdk.md) to listen directly from
your application using a standard `net.Listener`.

<p align="center">
  <img src="assets/images/overview.png" alt="overview" width="80%"/>
</p>

### HTTP(S)

Piko acts as a transparent HTTP(S) reverse proxy.

Incoming HTTP(S) requests identify the target endpoint to connect to using
either the `Host` header or `x-piko-endpoint` header.

When using the `Host` header, Piko uses the first segment as the endpoint ID.
Such as if your hosting Piko with a wildcard domain at `*.piko.example.com`,
sending a request to `foo.piko.example.com` will be routed to an upstream
listening on endpoint `foo`.

To avoid having to set up a wildcard domain you can instead use the
`x-piko-endpoint` header, such as if Piko is hosted at `piko.example.com`, you
can send requests to endpoint `foo` using header `x-piko-endpoint: foo`.

### TCP

Piko supports proxying TCP traffic, though unlike HTTP it requires using either
[Piko forward](./docs/forward/forward.md) or the
[Go SDK](./docs/sdk/go-sdk.md) to map the desired local TCP port to the target
endpoint (as there's no way to identify the target endpoint using raw TCP).

[Piko forward](./docs/forward/forward.md) is basically the reverse of
[Piko agent](./docs/agent/agent.md). Instead of listening on an endpoint and
forwarding to a local port on the upstream, Piko forward runs on the client and
listens on a TCP port then forwards the connection to the configured endpoint.

Such as using to listen on port `3000` and forward the connection to endpoint `my-endpoint`:
```
piko forward 3000 my-endpoint
```

You can also use the [Go SDK](./docs/sdk/go-sdk.md) to open a `net.Conn` that's
connected to the configured endpoint.

## Design Goals

### Production Traffic

Piko is built to serve production traffic, which means the Piko server must run
as a cluster of nodes to be fault tolerant, scale horizontally and support zero
downtime deployments.

Say an upstream is listening for traffic on endpoint E and connects to node N.
Node N will notify the other nodes that it has a listener for endpoint E, so
they can route incoming traffic for that endpoint to node N, which then
forwards the traffic to the upstream via its outbound-only connection to the
server. If node N fails or is deprovisioned, the upstream listener will
reconnect to another node and the cluster propagates the new routing
information to the other nodes in the cluster. See
[How Piko Works](./docs/how-piko-works.md) for details.

Piko also has a Prometheus endpoint, access logging, and status API so you can
monitor your deployment and debug issues. See observability for details.

### Hosting

Piko is built to be simple to host on Kubernetes. This means it can run as a
cluster of nodes (such as a StatefulSet), supports gradual rollouts, and can be
hosted behind a HTTP load balancer or Kubernetes Gateway.

Upstream services and downstream clients may connect to any node in the cluster
via the load balancer, then the cluster manages routing traffic to the
appropriate upstream.

See [Kubernetes](./docs/manage/kubernetes.md) for details.

## Getting Started

See [Getting Started](./docs/getting-started.md).

## How Piko Works

See [How Piko Works](./docs/how-piko-works.md).

## Support

Use [GitHub Discussions](https://github.com/andydunstall/piko/discussions) to
ask questions, get help, or suggest ideas.

## Docs

- [How Piko Works](./docs/how-piko-works.md)
- Tutorials
  - [Getting Started](./docs/getting-started.md)
  - [Install](./docs/tutorials/install.md)
  - [TCP Forwarding](./docs/tutorials/tcp-forwarding.md)
- [Server](./docs/server/server.md)
  - [Observability](./docs/server/observability.md)
  - [Kubernetes](./docs/server/kubernetes.md)
- [Agent](./docs/agent/agent.md)
- [Forward](./docs/forward/forward.md)
- [Go SDK](./docs/sdk/go-sdk.md)

## Contributing

See [CONTRIBUTING](./CONTRIBUTING.md).

## License
MIT License, please see [LICENSE](LICENSE) for details.
