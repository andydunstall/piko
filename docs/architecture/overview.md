# Overview

This document describes Pico’s architecture at a high level.

The Pico server runs as a cluster of nodes. Upstream services then create
outbound-only connections to the server and register endpoints. Proxy clients
send requests to the Pico server, which will forward requests to the
appropriate upstream service via its outbound-only connection.

A Pico cluster is designed to run behind a load balancer where upstream
services and proxy clients connect to random nodes in the cluster. Therefore
each node manages routing requests to another node with a connection to the
target upstream.

Upstreams register an endpoint ID they want to receive requests for. Pico then
manages proxying requests for that endpoint ID to an appropriate upstream. If
multiple upstreams have registered with the same endpoint ID, requests will be
load balanced among the available upstreams.

<p align="center">
  <img src="../../assets/images/overview.png" alt="overview" width="80%"/>
</p>

## Server

### Ports

The Pico server exposes 4 ports:
- Proxy port: Receives HTTP(S) requests from proxy clients which are routed to
an upstream service
- Upstream port: Accepts connections from upstream services
- Admin port: Exposes metrics and a status API to inspect the server state
- Gossip port: Used for inter-node gossip traffic

The proxy port and upstream port are kept separate to support different access
to each port. Such as if you're using Pico to access external customer
networks, the upstream port may be exposed to the Internet for upstreams to
connect, but you may only allow proxy requests from clients in the same network
as Pico. Similarly the admin port should not be exposed to the Internet.

### Cluster

Since Pico is designed to serve production traffic, it must be fault tolerant,
scalable and support zero-downtime updates. Therefore it should be hosted as a
cluster of nodes.

Nodes use gossip for cluster membership, failure detection and anti-entropy.
Each node maintains a local state containing metadata and the set of active
endpoints on that node (i.e. endpoints with at least one upstream connected to
the node). This state is propagated to the other nodes in the cluster, so each
node has an eventually consistent view of the other nodes and their active
endpoints (i.e. the cluster state).

### Routing

Nodes use this cluster state to decide which node to route incoming route
requests to. Say an upstream is connected to node N<sub>1</sub> and registered
endpoint E, then node N<sub>2</sub> receives a request for endpoint E, it will
route the request to N<sub>1</sub>.

Requests identify the endpoint ID to forward to in either the `Host` or
`x-pico-endpoint` header. If `Host` is used, the bottom level domain is used as
the endpoint ID, such as `my-endpoint.pico.example.com` uses `my-endpoint` as
the endpoint ID. `x-pico-endpoint` will take precedence over `Host`.

<p align="center">
  <img src="../../assets/images/routing.png" alt="routing" width="40%"/>
</p>

## Upstreams

Upstream services open outbound-only connections to Pico and register an
endpoint ID. The connection is the 'tunnel' to Pico and is the only transport
that's used between Pico and the upstream.

They connect to the servers 'upstream' port (`8002` by default) using
WebSockets at path `/pico/v1/listener/<endpoint ID>`, specifying the endpoint
ID they are registering. WebSockets are used to work with HTTP load balancers.

Pico then uses a bi-directional RPC layer built on top of WebSocket to send
requests to the upstream and receive responses. Each request/response has a
unique ID meaning they can be interleaved. Such as Pico may send requests A, B
then C, but receive responses B, C, then A.

Currently the easiest way to add an upstream service is using the Pico agent.
The agent is a lightweight proxy that runs alongside your service, that opens a
connection to Pico, registers the configured endpoints, then forwards incoming
requests to your service.
