# Getting Started

This example uses `docker-compose` to deploy three Pico server nodes behind
a load balancer, then registers endpoints to forward requests to a local HTTP
server.

## Server

Start by cloning Pico and building the Pico Docker image:
```shell
git clone git@github.com:andydunstall/pico.git
make pico
make image
```

This will build the Pico binary at `bin/pico` and Docker image `pico:latest`.

Next start the Pico server cluster:
```shell
cd docs/demo
docker compose up
```

This creates a cluster of three server nodes and a load balancer that exposes
the Pico ports. It will also start Prometheus and Grafana to inspect the Pico
metrics. The following ports are exposed:
- Pico proxy (`http://localhost:8000`): Forwards proxy requests to registered
endpoints
- Pico upstream (`http://localhost:8001`): Accepts connections from upstream
endpoints
- Pico admin (`http://localhost:8002`): Accepts admin connections for metrics,
the status API, and health checks
- Prometheus (`http://localhost:9090`)
- Grafana (`http://localhost:3000`)

To verify Pico has started correctly, run `pico status netmap nodes` which
queries the Pico admin API for the set of known Pico nodes.

## Agent

The Pico agent is a lightweight proxy that runs alongside your upstream
services. It connects to the Pico server and registers endpoints, then forwards
incoming requests to your services.

Create a simple HTTP server to forward requests to, such as
`python3 -m http.server 4000` so serve the files in the local directory on port
`4000`.

Then run the Pico agent and register endpoint `my-endpoint` using:
```shell
pico agent --endpoints my-endpoint/localhost:4000
```

See `pico agent -h` for the available options.

You can verify the endpoint has connected, again run `pico status netmap nodes`
and you'll see one of the Pico server nodes reporting endpoint `my-endpoint`
has an active connection.

### Request

As described above, when sending a request to Pico you can identify the
endpoint ID using either the `Host` header or the `x-pico-agent`.

Therefore to send a request to the registered endpoint, `my-endpoint`, use:
```shell
# x-pico-endpoint
curl http://localhost:8000 -H "x-pico-endpoint: my-endpoint"

# Host
curl --connect-to my-endpoint.example.com:8000:localhost:8000 http://my-endpoint.example.com:8000
```

This request will be load balanced among the Pico servers, then forwarded to
the endpoint registered above.
