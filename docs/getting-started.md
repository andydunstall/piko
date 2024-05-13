# Getting Started

To quickly get started with Pico, this example uses `docker compose` to deploy
a cluster of three Pico server nodes behind a load balancer. You may then
register upstream services to handle incoming requests proxied by Pico.

## Cluster

Start by cloning Pico and building the Pico Docker image:
```shell
git clone git@github.com:andydunstall/pico.git
make pico
make image
```

This will build the Pico binary at `bin/pico` and Docker image `pico:latest`.

Next start the Pico cluster:
```shell
cd docs/demo
docker compose up
```

This creates a cluster with three Pico server nodes and an NGINX load balancer.
The following ports are exposed:
* Proxy (`localhost:8000`): Forwards proxy requests to registered endpoints
* Upstream (`localhost:8001`): Accepts connections from upstream services
* Admin (`localhost:8002`): Handles admin requests for metrics, status API
and health checks

You can verify Pico has started and discovered the other nodes in the cluster
by running `pico status cluster nodes`, which will request the set of known
nodes from the Pico admin API (routed to a random node), such as:
```
$ pico status cluster nodes
nodes:
- id: pico-1-fuvaflv
  status: active
  proxy_addr: 172.18.0.3:8000
  admin_addr: 172.18.0.3:8002
  endpoints: 0
  upstreams: 0
- id: pico-2-rjtlyx1
  status: active
  proxy_addr: 172.18.0.4:8000
  admin_addr: 172.18.0.4:8002
  endpoints: 0
  upstreams: 0
- id: pico-3-p3wnt2z
  status: active
  proxy_addr: 172.18.0.2:8000
  admin_addr: 172.18.0.2:8002
  endpoints: 0
  upstreams: 0
```

You can also use the `--forward` flag to forward the request to a particular
node, such as `pico status cluster nodes --forward pico-3-p3wnt2z`.

The cluster also includes Prometheus and Grafana to inspect the cluster
metrics. You can open Grafana at `http://localhost:3000`.

## Agent

The Pico agent is a lightweight proxy that runs alongside your upstream
services. It connects to the Pico server and registers endpoints, then forwards
incoming requests to your services.

First create a local HTTP server to forward requests to, such as
`python3 -m http.server 4000` so serve the files in the local directory on port
`4000`.

Then run the Pico agent and register endpoint `my-endpoint` using:
```shell
pico agent --endpoints my-endpoint/localhost:4000
```

This will connect to the cluster load balancer, which routes the request to
a random Pico node. That outbound connection is then used to route requests
for that endpoint from Pico to the agent. Then agent will then forward the
request to your service.

See `pico agent -h` for the available options.

You can verify the upstream has connected and registered the endpoint by
running `pico status cluster nodes` again, which will now show one of the nodes
has a connected stream and registered endpoint:
```
$ pico status cluster nodes
nodes:
- id: pico-1-fuvaflv
  status: active
  proxy_addr: 172.18.0.3:8000
  admin_addr: 172.18.0.3:8002
  endpoints: 1
  upstreams: 1
- ...
```

You can also inspect the upstreams connected for that registered endpoint with
`pico status proxy endpoints --forward <node ID>`. Such as in the above example
the upstream is connected to node `pico-1-fuvaflv`:
```
$ pico status proxy endpoints --forward pico-1-fuvaflv
endpoints:
  my-endpoint:
  - 172.18.0.7:39084
```

### Request

When sending a request to Pico, you can identify the endpoint to route the
request to using either the `Host` or `x-pico-endpoint` header.

Therefore to send a request to the registered endpoint, `my-endpoint`, use
either:
```shell
# x-pico-endpoint
curl http://localhost:8000 -H "x-pico-endpoint: my-endpoint"

# Host
curl --connect-to my-endpoint.example.com:8000:localhost:8000 http://my-endpoint.example.com:8000
```

This request will be load balanced among the Pico servers, then forwarded to
the endpoint registered above.
