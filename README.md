# Pico [![Build](https://github.com/andydunstall/pico/actions/workflows/build.yaml/badge.svg)](https://github.com/andydunstall/pico/actions/workflows/build.yaml)

Pico is an open-source alternative to [Ngrok](https://ngrok.com/), designed to
serve production traffic and be simple to host (particularly on Kubernetes).
Such as you may use Pico to expose services in a customer network, a bring your
own cloud (BYOC) service or to connect to IoT devices.

The proxy server may be hosted as a cluster of nodes for fault tolerance, scale
and zero downtime deployments.

Upstream services connect to Pico and register endpoints. Pico will then route
requests for an endpoint to a registered upstream service via its outbound-only
connection. This means you can expose your services without opening a public
port.

Incoming HTTP(S) requests identify the ID of the target endpoint using either
the `Host` header or an `x-pico-endpoint` header. If multiple upstream services
have registered the same endpoint, Pico load balances requests for that
endpoint among the registered upstreams.

<p align="center">
  <img src="assets/images/overview.png" alt="overview" width="80%"/>
</p>

## Contents

- [Design Goals](#design-goals)
- [Getting Started](#getting-started)
- [Docs](#docs)

## Design Goals

### Production Traffic

Pico is designed to serve production traffic rather than as a tool for testing
and development. Such as you could use Pico to:
* Access customer networks
* Build a bring your own cloud (BYOC) solution
* Access IoT devices

To support this, Pico may run as a cluster of nodes in order to be fault
tolerant, scale horizontally and support zero downtime deployments. It also has
observability tools for monitoring and debugging.

### Hosting

Pico is built to be simple to host on Kubernetes. A Pico cluster may be hosted
as a Kubernetes StatefulSet behind a HTTP load balancer or Kubernetes Gateway.

Upstream service connections and proxy client requests may be load balanced to
any node in the cluster and Pico will manage routing the requests to the
correct upstream.

### Secure

Upstream services connect to Pico via an outbound-only connection. Pico will
then route any requests to the upstream via that connection. Therefore the
upstream never has to open a port to listen for requests.

Pico supports authenticating upstream services before they can register
endpoints.

Since Pico can be self-hosted, you can host it in the same network as your
proxy clients so never accept requests from an external network. Such as you
may have authenticated upstream services register from the Internet over TLS,
then only provide an internal route for proxy clients in the same network as
Pico.

## Getting Started

See [Getting Started](./docs/getting-started.md).

## Docs

- [Getting Started](./docs/getting-started.md)
- Architecture
  - [Overview](./docs/architecture/overview.md)
- Manage
  - [Configure](./docs/configure.md)
  - [Kubernetes](./docs/deploy/kubernetes.md)
  - [Observability](./docs/deploy/observability.md)
