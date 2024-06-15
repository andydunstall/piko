package client_test

import (
	"context"
	"fmt"
	"net/http"

	piko "github.com/andydunstall/piko/agent/client"
)

func handler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintln(w, "Hello from Piko")
}

func Example() {
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
