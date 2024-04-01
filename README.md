# Pico

> :warning: Pico is still a proof of concept so is not yet suitable for
production. See 'Limitations' below.

Pico is a reverse proxy that allows you to expose a service that isn't publicly
routable (known as tunnelling). Pico is designed to serve production traffic.

Upstream services register a listener with Pico via an outbound-only
connection. Downstream clients may then send HTTP(S) requests to Pico which
will be routed to an upstream listener.

Listeners are identified by an endpoint ID. Incoming HTTP requests include the
endpoint ID to route to in either the `Host` header or an `x-pico-endpoint`
header, then Pico load balances requests among registered listeners with that
endpoint ID.

![overview](assets/images/overview.png)

- [Design Goals](#design-goals)
- [Getting Started](#getting-started)
- [Docs](#docs)
- [Limitations](#limitations)

## Design Goals

### Production Traffic
Unlike most open-source tunnelling solutions that are built for testing and
development (such as sharing a demo running on your local machine), Pico is
built to serve production traffic. Such as it could be used to access a
customer network in a bring your own cloud (BYOC) service.

Pico supports running the server as a cluster of Pico nodes, meaning Pico is
fault tolerant and scales horizontally.

### Hosting
Pico is designed to be simple to host, particularly in Kubernetes. Therefore
Pico may be hosted behind a HTTP load balancer or
[Kubernetes Gateway](https://kubernetes.io/docs/concepts/services-networking/gateway/).

The downside of this approach is it means the proxy only supports HTTP. Pico
also uses WebSockets internally to communicate with upstream listeners, which
are typically supported by HTTP load balancers.

### Dynamic Endpoints
Upstream listeners may register any endpoint ID dynamically at runtime, without
any static configuration. When multiple listeners register with the same
endpoint ID, Pico will load balance requests among those listeners.

## Getting Started
Pico contains a server and agent.

The Pico server is responsible for accepting connections from both upstream
listeners and downstream clients. It then routes requests to the registered
listener.

Pico agent registers listeners with the Pico server then forwards requests from
Pico to your service.

Pico server should always be routable by both the upstream listeners and
downstream clients. It should also be hosted as a cluster of nodes for fault
tolerance. The Pico agent should be hosted alongside your service and must not
be routable by downstream clients.

### Build
To run Pico, either download from the releases page or build locally with
`make pico`, which will output `build/pico`. You must have Go 1.21 or later
installed.

### Running Locally
To get started quickly you can run both the Pico server and Pico agent locally.

Start a Pico server with `pico server`, which runs at `localhost:8080` by
default.

Next start a service you would like to route requests to, such as
`python3 -m http.server 3000` to create a simple HTTP server listening on port
`3000`.

Finally start the Pico agent with `pico agent my-endpoint-123 localhost:3000`
which will register a listener with the server for endpoint `my-endpoint-123`
and forward requests to `localhost:3000`.

You can then send requests to Pico which will be routed to the local service.
In production you can configure Pico to use the request `Host` header to route
requests, such as `my-endpoint-123.mypico.com`, though that requires setting up
wildcard DNS. Alternatively you can simply use the `x-pico-endpoint` header
whose value includes the endpoint ID, such as
`curl http://localhost:8080 -H "x-pico-endpoint: my-endpoint-123"`.

See `pico server -h` and `pico agent -h` for full configuration options.

### Running In Production
See [docs/hosting](docs/hosting) for details on running Pico in
production.

## Docs
See [docs](./docs) for details on deploying and managing Pico, plus details on
the Pico architecture.

## Limitations
> :warning: Pico is still a proof of concept so is not yet suitable for
production.

Pico does not yet support clustering or authentication.

Pico also only supports using Pico agent to register listeners, though aiming
to add support for a Go SDK as well.
