package proxy

import (
	"context"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
)

type LoadBalancer struct {
	server *http.Server
}

func NewLoadBalancer(addrs func() []string) *LoadBalancer {
	handler := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"

			addrs := addrs()
			req.URL.Host = addrs[rand.Int()%len(addrs)]
		},
	}
	return &LoadBalancer{
		server: &http.Server{
			Handler: handler,
		},
	}
}

func (lb *LoadBalancer) Serve(ln net.Listener) error {
	return lb.server.Serve(ln)
}

func (lb *LoadBalancer) Shutdown(ctx context.Context) error {
	return lb.server.Shutdown(ctx)
}
