<p align="center">
  <img src="assets/images/logo.png?raw=true" width='50%'>
</p>

---

Piko is an open-source alternative to [Ngrok](https://ngrok.com/), designed to
serve production traffic and be simple to host (particularly on Kubernetes).
Such as you may use Piko to expose services in a customer network, a bring your
own cloud (BYOC) service or to connect to IoT devices.

The proxy server may be hosted as a cluster of nodes for fault tolerance, scale
and zero downtime deployments.

Upstream services connect to Piko and register endpoints. Piko will then route
requests for an endpoint to a registered upstream service via its outbound-only
connection. This means you can expose your services without opening a public
port.

Incoming HTTP(S) requests identify the ID of the target endpoint using either
the `Host` header or an `x-piko-endpoint` header. If multiple upstream services
have registered the same endpoint, Piko load balances requests for that
endpoint among the registered upstreams.

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

Piko is designed to serve production traffic rather than as a tool for testing
and development. Such as you could use Piko to:
* Access customer networks
* Build a bring your own cloud (BYOC) solution
* Access IoT devices

To support this, Piko may run as a cluster of nodes in order to be fault
tolerant, scale horizontally and support zero downtime deployments. It also has
observability tools for monitoring and debugging.

### Hosting

Piko is built to be simple to host on Kubernetes. A Piko cluster may be hosted
as a Kubernetes StatefulSet behind a HTTP load balancer or Kubernetes Gateway.

Upstream service connections and proxy client requests may be load balanced to
any node in the cluster and Piko will manage routing the requests to the
correct upstream.

### Secure

Upstream services connect to Piko via an outbound-only connection. Piko will
then route any requests to the upstream via that connection. Therefore the
upstream never has to open a port to listen for requests.

Piko supports authenticating upstream services before they can register
endpoints.

Since Piko can be self-hosted, you can host it in the same network as your
proxy clients so never accept requests from an external network. Such as you
may have authenticated upstream services register from the Internet over TLS,
then only provide an internal route for proxy clients in the same network as
Piko.

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
