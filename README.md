<p align="center">
  <img src="assets/images/logo.png?raw=true" width='40%'>
</p>

---

Piko is a reverse proxy for accessing environments that aren’t publicly
routable, designed to be an open-source alternative to
[Ngrok](https://ngrok.com/).

Piko is built to serve production traffic and be simple to self-host on
Kubernetes. Such as you may use Piko to expose services in a customer network,
a bring your own cloud (BYOC) service or to connect to user devices.

Upstream services open an outbound connection to a Piko server and listen for
traffic on a particular endpoint. Piko then manages routing incoming
connections and requests to an upstream service listening on the target
endpoint. All traffic forwarded to the upstream is sent via its outbound-only
connection to Piko, so you never have to expose a port on the upstream.

Incoming HTTP(s) requests identify the target endpoint to connect to using
either the `Host` header or `x-piko-endpoint` header. Such as if an upstream is
listening on endpoint ‘my-service’, you can send requests to
‘my-service.piko.example.com’ or add a ‘x-endpoint-id: my-service’ header. If
multiple upstreams are listening on the same endpoint, Piko load balances
requests among those upstreams.

Since Piko is designed to serve production traffic, the server may be hosted as
a cluster of nodes, meaning it is fault tolerant, scales horizontally and
supports gradual rollouts.

<p align="center">
  <img src="assets/images/overview.png" alt="overview" width="80%"/>
</p>

## Contents

- [Design Goals](#design-goals)
- [Getting Started](#getting-started)
- [Support](#support)
- [Docs](#docs)
- [Contributing](#contributing)
- [License](#license)

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

### Dynamic Endpoints
Endpoints in Piko are the equivalent of a domain name in DNS.

Upstreams may listen to any endpoint and Piko load balances traffic for each
endpoint among the upstreams listening for that endpoint.

There is no static configuration required to configure endpoints, nor are
upstreams assigned random endpoint IDs. Instead upstreams can select their own
endpoint at runtime.

You may have multiple upstreams listening on the same endpoint, such as nodes
nodes in a payments service listening on the ‘payments’ endpoint, or each
upstream may have a unique endpoint identifying the node.

### Secure

Upstream services connect to the Piko server via an outbound-only connection.
Piko will then route traffic to the upstream via that connection. Therefore the
upstream never has to expose a port to listen for traffic.

The server has separate ports for traffic from downstream clients (the ‘proxy
port’) and connections from upstreams (the ‘upstream port’). In a typical
deployment you may expose the upstream port to the Internet for upstreams in
different networks to connect (using TLS and authentication), though only allow
proxy port access from within the same network as the server. Therefore you
never have to accept requests from external networks.

## Getting Started

See [Getting Started](./docs/getting-started.md).

## Support

Use [GitHub Discussions](https://github.com/andydunstall/piko/discussions) to
ask questions, get help, or suggest ideas.

## Docs

- [Getting Started](./docs/getting-started.md)
- Architecture
  - [Overview](./docs/architecture/overview.md)
- Manage
  - [Overview](./docs/manage/overview.md)
  - [Configure](./docs/manage/configure.md)
  - [Kubernetes](./docs/manage/kubernetes.md)
  - [Observability](./docs/manage/observability.md)

## Contributing

See [CONTRIBUTING](./CONTRIBUTING.md).

## License
MIT License, please see [LICENSE](LICENSE) for details.
