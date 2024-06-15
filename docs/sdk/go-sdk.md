# Go SDK

The Go SDK lets you open an upstream listener directly from your application.

Opening a listener returns a `net.Listener` which you can use as through you
opened a normal TCP listener.

Such as you can use the listener to create a HTTP server:
```go
package main

import (
	"context"
	"fmt"
	"net/http"

	piko "github.com/andydunstall/piko/agent/client"
)

func main() {
	var opts []piko.Option
	// ...

	client := piko.New(opts...)
	ln, err := client.Listen(context.Background(), "my-endpoint")
	if err != nil {
		panic("listen: " + err.Error())
	}

	if err := http.Serve(ln, http.HandlerFunc(handler)); err != nil {
		panic("http serve: " + err.Error())
	}
}


func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello from Piko")
}
```

See [`options.go`](../../agent/client/options.go) for the available options.
