# Getting Started

To quickly get started with Piko, this example uses `docker compose` to deploy
a cluster of three Piko server nodes behind a load balancer. You may then
register upstream services to handle incoming requests proxied by Piko.

## Cluster

Start by cloning Piko and building the Piko Docker image:
```shell
git clone git@github.com:andydunstall/piko.git
cd piko
make piko
make image
```

This will build the Piko binary at `bin/piko` and Docker image `piko:latest`.

Next start the Piko cluster:
```shell
cd docs/demo
docker compose up
```

This creates a cluster with three Piko server nodes and an NGINX load balancer.
The following ports are exposed:
* Proxy (`localhost:8000`): Forwards proxy requests to registered endpoints
* Upstream (`localhost:8001`): Accepts connections from upstream services
* Admin (`localhost:8002`): Handles admin requests for metrics, status API
and health checks

You can verify Piko has started and discovered the other nodes in the cluster
by running `piko status cluster nodes`, which will request the set of known
nodes from the Piko admin API (routed to a random node), such as:
```
$ piko status cluster nodes
nodes:
- id: piko-1-fuvaflv
  status: active
  proxy_addr: 172.18.0.3:8000
  admin_addr: 172.18.0.3:8002
  endpoints: 0
  upstreams: 0
- id: piko-2-rjtlyx1
  status: active
  proxy_addr: 172.18.0.4:8000
  admin_addr: 172.18.0.4:8002
  endpoints: 0
  upstreams: 0
- id: piko-3-p3wnt2z
  status: active
  proxy_addr: 172.18.0.2:8000
  admin_addr: 172.18.0.2:8002
  endpoints: 0
  upstreams: 0
```

You can also use the `--forward` flag to forward the request to a particular
node, such as `piko status cluster nodes --forward piko-3-p3wnt2z`.

The cluster also includes Prometheus and Grafana to inspect the cluster
metrics. You can open Grafana at `http://localhost:3000`.

## Agent

The Piko agent is a lightweight proxy that runs alongside your upstream
services. It connects to the Piko server and registers endpoints, then forwards
incoming requests to your services.

First create a local HTTP server to forward requests to, such as
`python3 -m http.server 4000` so serve the files in the local directory on port
`4000`.

Then run the Piko agent and register endpoint `my-endpoint` using:
```shell
piko agent --endpoints my-endpoint/localhost:4000
```

This will connect to the cluster load balancer, which routes the request to
a random Piko node. That outbound connection is then used to route requests
for that endpoint from Piko to the agent. Then agent will then forward the
request to your service.

See `piko agent -h` for the available options.

You can verify the upstream has connected and registered the endpoint by
running `piko status cluster nodes` again, which will now show one of the nodes
has a connected stream and registered endpoint:
```
$ piko status cluster nodes
nodes:
- id: piko-1-fuvaflv
  status: active
  proxy_addr: 172.18.0.3:8000
  admin_addr: 172.18.0.3:8002
  endpoints: 1
  upstreams: 1
- ...
```

You can also inspect the upstreams connected for that registered endpoint with
`piko status proxy endpoints --forward <node ID>`. Such as in the above example
the upstream is connected to node `piko-1-fuvaflv`:
```
$ piko status proxy endpoints --forward piko-1-fuvaflv
endpoints:
  my-endpoint:
  - 172.18.0.7:39084
```

### Request

When sending a request to Piko, you can identify the endpoint to route the
request to using either the `Host` or `x-piko-endpoint` header.

Therefore to send a request to the registered endpoint, `my-endpoint`, use
either:
```shell
# x-piko-endpoint
curl http://localhost:8000 -H "x-piko-endpoint: my-endpoint"

# Host
curl --connect-to my-endpoint.example.com:8000:localhost:8000 http://my-endpoint.example.com:8000
```

This request will be load balanced among the Piko servers, then forwarded to
the endpoint registered above.
