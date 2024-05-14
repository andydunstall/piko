# Overview

This document provides an overview on setting up and managing a Pico
deployment.

Also see [Architecture Overview](../architecture/overview.md).

## Server
The Pico server is responsible for accepting outbound-only connections from
upstream services, then routing requests from proxy clients to the appropriate
upstream.

The server is designed to be hosted as a cluster of nodes behind a HTTP load
balancer.

Run the a server node using `pico server`.

### Port

The Pico server exposes 4 ports:
* Proxy port: Receives HTTP(S) requests from proxy clients which are routed to
an upstream service
* Upstream port: Accepts connections from upstream services
* Admin port: Exposes metrics and a status API to inspect the server state
* Gossip port: Used for inter-node gossip traffic

The proxy port and upstream port are kept separate to support different access
to each port. Such as if you're using Pico to access external customer
networks, the upstream port may be exposed to the Internet for upstreams to
connect, but you may only allow proxy requests from clients in the same network
as Pico. Similarly the admin port should not be exposed to the Internet.

The proxy, upstream and admin ports are all designed to be hosted behind a HTTP
load balancer. The upstream port uses WebSockets so you must ensure your load
balancer is configured correctly.

### Cluster

To deploy Pico as a cluster, configure `--cluster.join` to list the addresses
of existing cluster members to join.

The addresses may be either addresses of specific nodes, such as
`10.26.104.14`, or a domain name that resolves to the IP addresses of all nodes
in the cluster.

To deploy Pico to Kubernetes, you can create a headless service whose domain
resolves to the IP addresses of the pods in the service, such as
`pico.prod-pico-ns.svc.cluster.local`. When Pico starts, it will then attempt
to join the other pods in the cluster. See [Kubernetes](./kubernetes.md) for
details on hosting Pico on Kubernetes.

See [Configure](./configure.md) for details.

### Observability

Each server node has an admin port (`8003` by default) which includes
Prometheus metrics at `/metrics`, a health endpoint at `/health`, and a status
API at `/status`. The status API exposes endpoints for inspecting the status of
a server node, which is used by the `pico status` CLI.

See [Observability](./observability.md) for details.

## Upstreams

Upstream services open outbound-only connections to Pico and register an
endpoint ID. The connection is the ‘tunnel’ to Pico and is the only transport
that's used between Pico and the upstream.

To add an upstream service, use the Pico agent. The agent is a lightweight
proxy that runs alongside your services, that opens a connection to Pico,
registers the configured endpoints, then forwards incoming requests to your
service.

Such as you may configure the agent to register the endpoint `my-endpoint` then
forward requests to `localhost:4000` using
`pico agent –endpoints my-endpoint/localhost:4000`.

See [Configure](./configure.md) for details.

## Authentication

Pico supports authenticating upstream services using a
[JSON Web Token (JWT)](https://jwt.io/) provided by your application.

Pico supports HMAC, RSA, and ECDSA JWT algorithms, specifically HS256, HS384,
HS512, RSA256, RSA384, RSA512, EC256, EC384, and EC512.

You configure the Pico server with the secret key (HMAC) or public key (RSA and
EC), then Pico will verify upstream connections using a valid JWT. Then
configure the Pico agent with a JWT API key which it will use when connecting
to Pico.

The JWT can also include Pico specific claims, such as a list of endpoints the
upstream service is permitted to register.

See [Configure](./configure.md) for details.
