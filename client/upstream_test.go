package client_test

import (
	"context"
	"fmt"
	"net/http"

	piko "github.com/andydunstall/piko/client"
)

// ExampleUpstream listens on endpoint 'my-endpoint' and uses the listener in
// a HTTP server.
func ExampleUpstream() {
	upstream := &piko.Upstream{
		// ...
	}

	ln, err := upstream.Listen(context.Background(), "my-endpoint")
	if err != nil {
		panic("listen: " + err.Error())
	}

	handler := func(w http.ResponseWriter, _ *http.Request) {
		// nolint
		fmt.Fprintln(w, "Hello from Piko!")
	}
	// As ln in a standard net.Listener, it can be used in a HTTP server.
	if err := http.Serve(ln, http.HandlerFunc(handler)); err != nil {
		panic("serve: " + err.Error())
	}
}
