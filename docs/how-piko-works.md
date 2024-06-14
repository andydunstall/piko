# How Piko Works

This document provides an overview of how Piko works.

The [Piko server](./server/server.md) is a HTTP(S) reverse proxy that forwards
requests to upstream listeners. Unlike a traditional reverse proxy, Piko never
opens a connection directly to your upstream. Instead upstreams listeners open
outbound-only connections to the server and listen on a particular endpoint.
The server then forwards requests to a listening upstream via its outbound
connection to the server.

This means upstreams can run anywhere without requiring a public route, as long
as they can open a connection to the Piko server.

You can create an upstream listener using the [Piko agent](./agent/agent.md).

## Upstream

Each upstream listener opens an outbound-only connection to the Piko server and
specifies what endpoint it is listening on. This connection is used to forward
requests from the server to the upstream.

To make the server easier to host, the upstream listener connection uses
WebSockets rather than raw TCP, meaning the server can be deployed behind a
HTTP load balancer.

Piko multiplexes parallel bi-directional streams on this underlying connection
using [yamux](https://github.com/hashicorp/yamux). Each stream can be treated
as a separate TCP connection even though there is actually only one underlying
connection. This means the server can "connect" to the upstream by opening a
new stream on the existing connection.

When the server receives a request from a downstream client for a particular
endpoint, it looks up a connected upstream listener for that endpoint, opens a
new stream, then forwards the request and response. Piko also supports long
lived connections like WebSockets, where it relays bytes between the downstream
client and upstream listener over this stream.

If an upstream is disconnected it will automatically reconnect and resume
listening on the endpoint.

## Cluster

To be fault tolerant and scalable, the Piko server is designed to be hosted as
a cluster of nodes.

Since the cluster is typically hosted behind a HTTP load balancer, upstream
listeners and downstream clients may connect to any node in the cluster.

Say you have an upstream connected to node N<sub>1</sub> listening on endpoint
E. If a downstream client sends a request to node N<sub>2</sub> for that
endpoint, Piko must manage routing the request to the upstream connected to
N<sub>1</sub>.

<p align="center">
  <img src="../assets/images/routing.png" alt="overview" width="30%"/>
</p>

When a Piko node receives a request for endpoint E, it first looks up whether
it has a connected upstream listener for that endpoint. If it does, the node
opens a new stream to the upstream listener and forwards the request, as
described above.

If the node doesn’t have a connected upstream listener for the endpoint, it
looks up whether another node in the cluster has an upstream for the endpoint.
If found, the request is forwarded to that target node, then that node can
forward the request to the upstream via its connection to the node.

When there are multiple upstream listeners connected for an endpoint, requests
are load balanced among those upstreams.

This approach requires each node knows what endpoints each other node has a
connected upstream listener for. Piko uses an efficient gossip-based
anti-entropy mechanism to quickly propagate this state. When the set of
connected upstreams to a node changes (due to upstreams connecting or
disconnecting), the update will be quickly propagated around the cluster
(usually in less than a second).
