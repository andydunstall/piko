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
their outbound connections.

Piko has two key design goals:
* Serve production traffic: To serve production traffic Piko may run as a
cluster of nodes for fault tolerance, horizontal scaling and zero-downtime
deployments
* Simple to host: Piko is designed to be simple to self-host, particularly on
Kubernetes

Such as you may use Piko to expose services in a customer network, a bring your
own cloud (BYOC) service, or to connect to user devices.

Therefore Piko can be used as an open-source alternative to
[Ngrok](https://ngrok.com/).

### Reverse Proxy

In a traditional reverse proxy, you configure routing rules describing how to
route incoming traffic to upstream services. The proxy will then open
connections to your services and forward incoming traffic. This means your
upstream services must be discoverable and have an exposed port that's
accessible from the proxy.

With Piko, rather than configuring routing rules in the proxy, instead your
upstreams configure their own routing rules and open a secure outbound-only
connection to Piko. Piko then forwards incoming traffic to the correct upstream
via its outbound connection.

Therefore your services may run anywhere without requiring a public route, as
long as they can open a connection to Piko. This enables accessing services in
private environments, such as an external customer network or your local
network It can also be used to simplify your infrastructure as you don’t need
to set up firewall rules, DNS, certificates, load balancers…

### Endpoints

Upstream services listen for traffic on a particular endpoint. Piko then
manages routing incoming connections and requests to an upstream service
listening on the target endpoint. If multiple upstreams are listening on the
same endpoint, requests are load balanced among the available upstreams.

No static configuration is required to configure endpoints, upstreams can
listen on any endpoint they choose.

Incoming HTTP(S) requests identify the target endpoint to connect to using
either the `Host` header or `x-piko-endpoint` header.

When using the `Host` header, Piko uses the first segment as the endpoint ID.
Such as if your hosting Piko with a wildcard domain at `*.piko.example.com`,
sending a request to `foo.piko.example.com` will be routed to an upstream
listening on endpoint `foo`.

To avoid having to set up a wildcard domain you can instead use
`x-piko-endpoint`, such as if Piko is hosted at `piko.example.com`, you can
send requests to endpoint `foo` using header `x-piko-endpoint: foo`.

<p align="center">
  <img src="assets/images/overview.png" alt="overview" width="80%"/>
</p>


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
[Architecture Overview](./docs/architecture/overview.md) for details.

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
- [Server](./docs/server/server.md)
  - [Observability](./docs/server/observability.md)
  - [Kubernetes](./docs/server/kubernetes.md)
- [Agent](./docs/agent/agent.md)

## Contributing

See [CONTRIBUTING](./CONTRIBUTING.md).

## License
MIT License, please see [LICENSE](LICENSE) for details.
