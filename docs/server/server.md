# Server

The Piko server is responsible for forwarding traffic from downstream clients
to upstream listeners. See [How Piko Works](../how-piko-works.md) for details.

The server is designed to be hosted as a cluster of nodes behind a HTTP load
balancer.

Run a server node with `piko server`.

## Ports

The Piko server exposes 4 ports:
* Proxy port: Receives HTTP(S) requests from proxy clients which are routed to
an upstream service
* Upstream port: Accepts connections from upstream services
* Admin port: Exposes metrics and a status API to inspect the server state
* Gossip port: Used for inter-node gossip traffic

The proxy port and upstream port are kept separate to support different access
to each port. Such as if you're using Piko to access external customer
networks, the upstream port may be exposed to the Internet for upstreams to
connect, but you may only allow proxy requests from clients in the same network
as Piko. Similarly the admin port should not be exposed to the Internet.

The proxy, upstream and admin ports are all designed to be hosted behind a HTTP
load balancer. The upstream port uses WebSockets so you must ensure your load
balancer is configured correctly.

## Configuration

Piko server supports both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

See `piko server -h` for the available configuration options.

### Variable Substitution

When enabling `--config.expand-env`, Piko will expand environment variables
in the loaded YAML configuration. This will replace references to `${VAR}`
and `$VAR` with the corresponding environment variable.

If the environment variable is not defined, it will be replaced with an empty
string. You can also define a default value using form `${VAR:default}`.

### YAML Configuration

The server supports the following YAML configuration (where most parameters
have corresponding command line flags):

```
cluster:
  # A unique identifier for the node in the cluster.
  #
  # By default a random ID will be generated for the node.
  node_id: ""

  # A prefix for the node ID.
  #
  # Piko will generate a unique random identifier for the node and append it to
  # the given prefix.
  #
  # Such as you could use the node or pod  name as a prefix, then add a unique
  # identifier to ensure the node ID is unique across restarts.
  node_id_prefix: ""

  # A list of addresses of members in the cluster to join.
  #
  # This may be either addresses of specific nodes, such as
  # '--cluster.join 10.26.104.14,10.26.104.75', or a domain that resolves to
  # the addresses of the nodes in the cluster (e.g. a Kubernetes headless
  # service), such as '--cluster.join piko.prod-piko-ns'.
  #
  # Each address must include the host, and may optionally include a port. If no
  # port is given, the gossip port of this node is used.
  #
  # Note each node propagates membership information to the other known nodes,
  # so the initial set of configured members only needs to be a subset of nodes.
  join: []

  # Whether the server node should abort if it is configured with more than one
  # node to join (excluding itself) but fails to join any members.
  abort_if_join_fails: true

proxy:
  # The host/port to listen for incoming proxy connections.
  #
  # If the host is unspecified it defaults to all listeners, such as
  # '--proxy.bind-addr :8000' will listen on '0.0.0.0:8000'.
  bind_addr: ":8000"

  # Proxy to advertise to other nodes in the cluster. This is the
  # address other nodes will used to forward proxy connections.
  #
  # Such as if the listen address is ':8000', the advertised address may be
  # '10.26.104.45:8000' or 'node1.cluster:8000'.
  #
  # By default, if the bind address includes an IP to bind to that will be used.
  # If the bind address does not include an IP (such as ':8000') the nodes
  # private IP will be used, such as a bind address of ':8000' may have an
  # advertise address of '10.26.104.14:8000'.
  advertise_addr: ""

  # Timeout when forwarding incoming requests to the upstream.
  timeout: 30s

  # Whether to log all incoming connections and requests.
  access_log: true

  http:
    # The maximum duration for reading the entire request, including the body. A
    # zero or negative value means there will be no timeout.
    read_timeout: 10s

    # The maximum duration for reading the request headers. If zero,
    # http.read-timeout is used.
    read_header_timeout: 10s

    # The maximum duration before timing out writes of the response.
    write_timeout: 10s

    # The maximum amount of time to wait for the next request when keep-alives are
    # enabled.
    idle_timeout: 5m0s

    # The maximum number of bytes the server will read parsing the request header's
    # keys and values, including the request line.
    max_header_bytes: 1048576

  tls:
    # Whether to enable TLS on the listener.
    #
    # If enabled must configure the cert and key.
    enabled: false

    # Path to the PEM encoded certificate file.
    cert: ""

    # Path to the PEM encoded key file.
    key: ""

upstream:
  # The host/port to listen for incoming upstream connections.
  #
  # If the host is unspecified it defaults to all listeners, such as
  # '--upstream.bind-addr :8001' will listen on '0.0.0.0:8001'.
  bind_addr: ":8001"

  tls:
    # Whether to enable TLS on the listener.
    #
    # If enabled must configure the cert and key.
    enabled: false

    # Path to the PEM encoded certificate file.
    cert: ""

    # Path to the PEM encoded key file.
    key: ""

gossip:
  # The host/port to listen for inter-node gossip traffic.
  #
  # If the host is unspecified it defaults to all listeners, such as
  # '--gossip.bind-addr :8003' will listen on '0.0.0.0:8003'.
  bind_addr: ":8003"

  # Gossip listen address to advertise to other nodes in the cluster. This is the
  # address other nodes will used to gossip with the node.
  #
  # Such as if the listen address is ':8003', the advertised address may be
  # '10.26.104.45:8003' or 'node1.cluster:8003'.
  #
  # By default, if the bind address includes an IP to bind to that will be used.
  # If the bind address does not include an IP (such as ':8003') the nodes
  # private IP will be used, such as a bind address of ':8003' may have an
  # advertise address of '10.26.104.14:8003'.
  advertise_addr: ""

  # The interval to initiate rounds of gossip.
  #
  # Each gossip round selects another known node to synchronize with.`,
  interval: 500ms

  # The maximum size of any packet sent.
  #
  # Depending on your networks MTU you may be able to increase to include more data
  # in each packet.
  max_packet_size: 1400

admin:
  # The host/port to listen for incoming admin connections.
  #
  # If the host is unspecified it defaults to all listeners, such as
  # '--admin.bind-addr :8002' will listen on '0.0.0.0:8002'.
  bind_addr: ":8002"

  # Admin listen address to advertise to other nodes in the cluster. This is the
  # address other nodes will used to forward admin requests.
  #
  # Such as if the listen address is ':8002', the advertised address may be
  # '10.26.104.45:8002' or 'node1.cluster:8002'.
  #
  # By default, if the bind address includes an IP to bind to that will be used.
  # If the bind address does not include an IP (such as ':8002') the nodes
  # private IP will be used, such as a bind address of ':8002' may have an
  # advertise address of '10.26.104.14:8002'.
  advertise_addr: ""

  tls:
    # Whether to enable TLS on the listener.
    #
    # If enabled must configure the cert and key.
    enabled: false

    # Path to the PEM encoded certificate file.
    cert: ""

    # Path to the PEM encoded key file.
    key: ""

auth:
    # Secret key to authenticate HMAC endpoint connection JWTs.
    token_hmac_secret_key: ""

    # Public key to authenticate RSA endpoint connection JWTs.
    token_rsa_public_key: ""

    # Public key to authenticate ECDSA endpoint connection JWTs.
    token_ecdsa_public_key: ""

    # Audience of endpoint connection JWT token to verify.
    #
    # If given the JWT 'aud' claim must match the given audience. Otherwise it
    # is ignored.
    token_audience: ""

    # Issuer of endpoint connection JWT token to verify.
    #
    # If given the JWT 'iss' claim must match the given issuer. Otherwise it
    # is ignored.
    token_issuer: ""

log:
    # Minimum log level to output.
    #
    # The available levels are 'debug', 'info', 'warn' and 'error'.
    level: info

    # Each log has a 'subsystem' field where the log occured.
    #
    # '--log.subsystems' enables all log levels for those given subsystems. This
    # can be useful to debug a particular subsystem without having to enable all
    # debug logs.
    #
    # Such as you can enable 'gossip' logs with '--log.subsystems gossip'.
    subsystems: []

# Maximum duration after a shutdown signal is received (SIGTERM or
# SIGINT) to gracefully shutdown the server node before terminating.
# This includes handling in-progress HTTP requests, gracefully closing
# connections to upstream listeners and announcing to the cluster the node is
# leaving.
grace_period: 1m0s
```

## Cluster

To deploy Piko as a cluster, configure `--cluster.join` to a list of cluster
members in the cluster to join.

The addresses may be either addresses of specific nodes, such as
`10.26.104.14`, or a domain name that resolves to the IP addresses of all nodes
in the cluster.

To deploy Piko to Kubernetes, you can create a headless service whose domain
resolves to the IP addresses of the pods in the service, such as
`piko.prod-piko-ns.svc.cluster.local`. When Piko starts, it will then attempt
to join the other pods in the cluster. See [Kubernetes](./kubernetes.md) for
details on hosting Piko on Kubernetes.

## Authentication

To authenticate upstream endpoint connections, Piko can use a
[JSON Web Token (JWT)](https://jwt.io/) provided by your application.

You configure Piko with the secret key or public key to verify the JWT, then
configure the upstream endpoint with the JWT. Piko will then verify endpoints
connection token.

Piko supports HMAC, RSA, and ECDSA JWT algorithms, specifically HS256, HS384,
HS512, RSA256, RSA384, RSA512, EC256, EC384, and EC512.

The server has the following configuration options to pass a secret key or
public key:
- `auth.token_hmac_secret_key`: Add HMAC secret key
- `auth.token_rsa_public_key`: Add RSA public key
- `auth.token_ecdsa_public_key`: Add ECDSA public key

If no keys secret or public keys are given, Piko will allow unauthenticated
endpoint connections.

Piko will verify the `exp` (expiry) and `iat` (issued at) claims if given, and
drop the connection to the upstream endpoint once its token expires.

By default Piko will not verify the `aud` (audience) or `iss` (issuer) claims,
though you can enable these checks with `auth.token_audience` and
`auth.token_issuer` respectively.

You may also include Piko specific fields in your JWT. Piko supports the
`piko.endpoints` claim which contains an array of endpoint IDs the token is
permitted to register. Such as if the JWT includes claim
`"piko": {"endpoints": ["endpoint-123"]}`, it will be permitted to register
endpoint ID `endpoint-123` but not `endpoint-xyz`.

Note Piko does (yet) not authenticate proxy requests as proxy clients will
typically be deployed to the same network as the Pcio server. Your upstream
services may then authenticate incoming requests if needed after they've been
forwarded by Piko.

## Observability

Each server node has an admin port (`8003` by default) which includes
Prometheus metrics at `/metrics`, a health endpoint at `/health`, and a status
API at `/status`. The status API exposes endpoints for inspecting the status of
a server node, which is used by the `piko server status` CLI.

See [Observability](./observability.md) for details.
