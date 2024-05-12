package proxy

import (
	"context"
	"net/http"
	"sync"

	"github.com/andydunstall/pico/pkg/log"
)

// localEndpoint contains the local connections for an endpoint ID.
type localEndpoint struct {
	conns     []Conn
	nextIndex int
}

func (e *localEndpoint) AddConn(c Conn) {
	e.conns = append(e.conns, c)
}

// RemoveConn removes the connection if it exists and returns whether there are
// any remaining connections fro the endpoint ID.
func (e *localEndpoint) RemoveConn(c Conn) bool {
	for i := 0; i != len(e.conns); i++ {
		if e.conns[i] != c {
			continue
		}
		e.conns = append(e.conns[:i], e.conns[i+1:]...)
		if len(e.conns) == 0 {
			return true
		}
		e.nextIndex %= len(e.conns)
		return false
	}
	return len(e.conns) == 0
}

// Next returns the next connection to the endpoint in a round-robin fashion.
func (e *localEndpoint) Next() Conn {
	if len(e.conns) == 0 {
		return nil
	}

	s := e.conns[e.nextIndex]
	e.nextIndex++
	e.nextIndex %= len(e.conns)
	return s
}

// localProxy is responsible for forwarding requests to upstream endpoints
// connected to the local node.
type localProxy struct {
	endpoints map[string]*localEndpoint

	mu sync.Mutex

	metrics *Metrics

	logger log.Logger
}

func newLocalProxy(metrics *Metrics, logger log.Logger) *localProxy {
	return &localProxy{
		endpoints: make(map[string]*localEndpoint),
		metrics:   metrics,
		logger:    logger,
	}
}

// Request attempts to forward the request to an upstream endpoint connected to
// the local node.
func (p *localProxy) Request(
	ctx context.Context,
	endpointID string,
	r *http.Request,
) (*http.Response, error) {
	conn := p.findConn(endpointID)
	if conn == nil {
		// No connection found.
		return nil, errEndpointNotFound
	}

	p.metrics.ForwardedLocalTotal.Inc()

	return conn.Request(ctx, r)
}

func (p *localProxy) AddConn(conn Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	e, ok := p.endpoints[conn.EndpointID()]
	if !ok {
		e = &localEndpoint{}

		p.metrics.RegisteredEndpoints.Inc()
	}

	e.AddConn(conn)
	p.endpoints[conn.EndpointID()] = e

	p.metrics.ConnectedUpstreams.Inc()
}

func (p *localProxy) RemoveConn(conn Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoint, ok := p.endpoints[conn.EndpointID()]
	if !ok {
		return
	}
	if endpoint.RemoveConn(conn) {
		delete(p.endpoints, conn.EndpointID())

		p.metrics.RegisteredEndpoints.Dec()
	}

	p.metrics.ConnectedUpstreams.Dec()
}

func (p *localProxy) ConnAddrs() map[string][]string {
	p.mu.Lock()
	defer p.mu.Unlock()

	c := make(map[string][]string)
	for endpointID, endpoint := range p.endpoints {
		for _, conn := range endpoint.conns {
			c[endpointID] = append(c[endpointID], conn.Addr())
		}
	}
	return c
}

func (p *localProxy) findConn(endpointID string) Conn {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoint, ok := p.endpoints[endpointID]
	if !ok {
		return nil
	}
	return endpoint.Next()
}
