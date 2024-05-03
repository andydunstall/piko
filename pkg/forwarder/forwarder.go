package forwarder

import (
	"context"
	"net/http"
)

// Forwarder handles forwarding the given HTTP request to a server.
type Forwarder interface {
	Request(ctx context.Context, addr string, r *http.Request) (*http.Response, error)
}

type forwarder struct {
	client *http.Client
}

func NewForwarder() Forwarder {
	return &forwarder{
		client: &http.Client{},
	}
}

func (f *forwarder) Request(
	ctx context.Context,
	addr string,
	r *http.Request,
) (*http.Response, error) {
	r = r.WithContext(ctx)

	r.URL.Scheme = "http"
	r.URL.Host = addr
	r.RequestURI = ""

	return f.client.Do(r)
}
