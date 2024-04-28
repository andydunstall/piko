# Kubernetes

Pico is designed to be easy to host as a cluster of nodes on Kubernetes. When
Pico is hosted as a cluster it will gracefully handle nodes leaving and
joining.

## Discovery

Pico is configured to join a set of known nodes in a cluster using
`--cluster.join`. Rather than maintain a static list of IP addresses, the
easiest option is to create a headless service for Pico. This will create a
DNS record that resolves to the addresses of each Pico pod.

You can then configure `--cluster.join` with this DNS record, such as
`pico.pico-ns.svc.cluster.local`, then when Pico starts it will attempt to
join any existing nodes.

## Ports

The proxy port accepts connections from downstream clients. It only
supports HTTP and defaults to port `8000`.

The upstream port accepts connections from upstream listeners via
WebSockets, so if you route using a HTTP load balancer or gateway, you must
ensure WebSockets are supported/enabled. It defaults to port `8001`.

The admin port accepts admin connections to inspect the status of the server.
This includes Prometheus metrics at `/metrics` and a status API at `/status`
which is used by the `pico status` CLI. It defaults to port `8002`.

Finally the gossip port is used for inter-node traffic to propagate the cluster
state which defaults to port `7000`.
