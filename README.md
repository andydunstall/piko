# Pico [![Build](https://github.com/andydunstall/pico/actions/workflows/build.yaml/badge.svg)](https://github.com/andydunstall/pico/actions/workflows/build.yaml)

> :warning: Pico is currently only a proof of concept so is not yet suitable
for production.

Pico is a reverse proxy that allows you to expose an endpoint that isn’t
publicly routable (known as tunnelling).

Unlike many open-source tunnelling solutions, Pico is designed to serve
production traffic. Such as you may use Pico to expose services in a customer
network, a bring your own cloud (BYOC) service or to connect to IoT devices.

Upstream endpoints register with Pico via an outbound-only connection. Proxy
clients then send HTTP(S) requests to Pico which will proxy the requests to a
registered endpoint.

Requests identify the target endpoint ID using either the `Host` header or an
`x-pico-endpoint` header. When multiple endpoints have the same ID, Pico will
load balance requests for that ID among the registered endpoints. Therefore an
endpoint ID is the equivalent of a domain name in Pico.

![overview](assets/images/overview.png)

## Contents

- [Design Goals](#design-goals)
- [Components](#components)
- [Getting Started](#getting-started)
- [Docs](#docs)
- [Limitations](#limitations)

## Design Goals

### Production Traffic
Unlike many open-source tunnelling solutions that are built for testing and
development (such as sharing a demo running on your local machine), Pico is
built to serve production traffic. Such as you could use Pico to:
* Access customer networks
* Build a bring your own cloud (BYOC) solution
* Access IoT devices

Therefore Pico supports running as a cluster of server nodes in order to be
fault tolerant, scale horizontally and support zero downtime deployments. It
also includes observability tools for monitoring, alerting and debugging.

### Hosting
Pico is designed to be simple to host, particularly on Kubernetes. A cluster of
server nodes can be hosted behind a HTTP load balancer or
[Kubernetes Gateway](https://kubernetes.io/docs/concepts/services-networking/gateway/)
as a Kubernetes deployment or stateful set.

Upstream endpoints and proxied requests may be load balanced to any node in the
cluster, then Pico will manage routing the request to the correct endpoint.

### Dynamic Endpoints
Upstreams may register any endpoint ID dynamically at runtime without any
static configuration. If there are multiple registered endpoints for a given
ID, Pico load balances requests among those endpoints.

## Components

### Server
The Pico server is responsible for proxying requests from proxy clients to
registered upstream endpoints.

Upstream endpoints register with Pico via an outbound-only connection. Clients
then send HTTP(S) requests to the Pico server, which will proxy the requests to
a registered endpoint.

Incoming requests identify the target endpoint ID using either the `Host`
header or `x-pico-endpoint` header. When the `Host` header is used, the lowest
level domain is used as the endpoint ID. Such as if you send a request to
`my-endpoint.pico.example.com`, `my-endpoint` will be used. `x-pico-endpoint`
takes precedence over the `Host`.

Pico supports running as a cluster of server nodes. Upstream listeners and
proxy clients may connect to any node in the cluster and Pico manages
routing requests to the correct listener.

The server exposes 4 ports:
* Proxy port: Listens for HTTP(S) requests from proxy clients and forwards
the requests to upstream listeners (defaults to `8000`)
* Upstream port: Listens for connections from upstream listeners (defaults to
`8001`)
* Admin port: Listens for admin requests to inspect the status of the server
(defaults to `8002`)
* Gossip port: Listens for gossip traffic between nodes in the same cluster
(defaults to `8003`)

The server has separate proxy and upstream ports as upstream listeners and
proxy clients will be in separate networks (otherwise there isn’t any need
for Pico). Such as you may expose the upstream port to the Internet for
external networks to register endpoints, though only allow requests to the
proxy port from nodes in the same network.

Run a server node with `pico server`.

### Agent
The Pico agent is a lightweight proxy that runs alongside your upstream
services that registers endpoints with the Pico server and forwards incoming
requests.

Such as you run an Pico agent and register endpoint `my-endpoint` that forwards
requests to `localhost:3000`.

Run the Pico agent with `pico agent`.

## Getting Started

See [Getting Started](./docs/getting-started.md).

## Docs

- [Getting Started](./docs/getting-started.md)
- [Architecture](./docs/architecture.md)
- [Configure](./docs/configure.md)
- Manage
  - [Kubernetes](./docs/deploy/kubernetes.md)
  - [Observability](./docs/deploy/observability.md)

## Limitations

Pico is currently only a proof of concept so is not yet suitable for
production. It likely contains bugs and is missing important features like
authentication.
