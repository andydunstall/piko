# Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/andydunstall/piko/client)](https://pkg.go.dev/github.com/andydunstall/piko/client)

The Go SDK lets you open upstream listeners or dial endpoints directly from
your application.

## Getting Started

### Upstream

This example creates a Piko upstream listening on endpoint `my-endpoint`
and listens for HTTP requests with a `http.Server`.

Opening a listener returns a `net.Listener` that you can use to accept
connections like a standard TCP listener.

Such as you can use the listener to create a HTTP server:
```go
package main

import (
	"context"
	"fmt"
	"net/http"

	piko "github.com/andydunstall/piko/client"
)

func main() {
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
```

### Dialer

To open a TCP connection to a Piko upstream, you must use either
[Piko forward](../forward/forward.md) or the Go SDK. This is needed to specify
the endpoint ID to connect to, which you can't do with 'raw TCP.

Note when using HTTP or WebSockets you can connect to Piko directly and include
the Piko endpoint to connect to in the HTTP request header. 

This example opens a connection to endpoint `my-endpoint` which returns a
`net.Conn`:

```go
package main

import (
	"context"

	piko "github.com/andydunstall/piko/client"
)

func main() {
	dialer := &piko.Dialer{
		// ...
	}
	conn, err := dialer.Dial(context.Background(), "my-endpoint")
	if err != nil {
		panic("dial: " + err.Error())
	}

	if _, err := conn.Write([]byte("hello")); err != nil {
		panic("write: " + err.Error())
	}
}
```

## Documentation

See [Go Reference](https://pkg.go.dev/github.com/andydunstall/piko/client) for
the full client documentation and examples.
