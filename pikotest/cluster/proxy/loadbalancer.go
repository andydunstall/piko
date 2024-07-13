package proxy

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	"github.com/andydunstall/piko/pikotest/cluster"
)

type reverseProxy struct {
	addr string

	server *http.Server
}

func newReverseProxy(addr string, upstreams func() []string) *reverseProxy {
	handler := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"

			upstreams := upstreams()
			req.URL.Host = upstreams[rand.Int()%len(upstreams)]
		},
	}
	return &reverseProxy{
		addr: addr,
		server: &http.Server{
			Handler: handler,
		},
	}
}

func (rp *reverseProxy) Serve() error {
	ln, err := net.Listen("tcp", rp.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	if err := rp.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (rp *reverseProxy) Close() error {
	return rp.server.Close()
}

// LoadBalancer is a load balancer for a local cluster.
type LoadBalancer struct {
	proxyServer *reverseProxy

	upstreamServer *reverseProxy

	adminServer *reverseProxy

	wg sync.WaitGroup
}

func NewLoadBalancer(manager *cluster.Manager) *LoadBalancer {
	proxyServer := newReverseProxy("127.0.0.1:8000", func() []string {
		var addrs []string
		for _, node := range manager.Nodes() {
			addrs = append(addrs, node.ProxyAddr())
		}
		return addrs
	})

	upstreamServer := newReverseProxy("127.0.0.1:8001", func() []string {
		var addrs []string
		for _, node := range manager.Nodes() {
			addrs = append(addrs, node.UpstreamAddr())
		}
		return addrs
	})

	adminServer := newReverseProxy("127.0.0.1:8002", func() []string {
		var addrs []string
		for _, node := range manager.Nodes() {
			addrs = append(addrs, node.AdminAddr())
		}
		return addrs
	})

	lb := &LoadBalancer{
		proxyServer:    proxyServer,
		upstreamServer: upstreamServer,
		adminServer:    adminServer,
	}

	lb.serve()

	return lb
}

func (lb *LoadBalancer) Close() {
	lb.proxyServer.Close()
	lb.upstreamServer.Close()
	lb.adminServer.Close()
	lb.wg.Wait()
}

func (lb *LoadBalancer) serve() {
	lb.wg.Add(1)
	go func() {
		defer lb.wg.Done()

		if err := lb.proxyServer.Serve(); err != nil {
			panic("serve: " + err.Error())
		}
	}()

	lb.wg.Add(1)
	go func() {
		defer lb.wg.Done()

		if err := lb.upstreamServer.Serve(); err != nil {
			panic("serve: " + err.Error())
		}
	}()

	lb.wg.Add(1)
	go func() {
		defer lb.wg.Done()

		if err := lb.adminServer.Serve(); err != nil {
			panic("serve: " + err.Error())
		}
	}()
}
