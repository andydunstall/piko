# Scale Targets

Pico is designed for a high number of active endpoints (i.e. an endpoint with
at least one upstream listener), though a low rate of requests per endpoint.
Such as Pico may be used for building a control plane sending control messages
to customer networks and instances, rather than hosting a public web server.

Therefore aiming to support:
* 25 Pico server nodes
* 10,000 upstream listeners: Since each upstream listener is typically a
separate instance, 10,000 listeners should be enough
* 10,000 active endpoints
* 500 listeners added and removed per second
* 10 1KB requests per second per listener

Note Pico is also easy to partition, such as you may have a Pico cluster per
region with its own load balancer (e.g. `pico.us-east-1.myapp.com`). Therefore
10,000 listeners per region should be more than enough in most cases.

These are just rough targets to start with while building cluster support so
can be updated later if needed.
