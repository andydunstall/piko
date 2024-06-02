package piko_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	piko "github.com/andydunstall/piko/client"
)

func Example() {
	pikoToken := os.Getenv("PIKO_TOKEN")

	// Connect to the Piko server.
	piko, err := piko.Connect(
		context.Background(),
		piko.WithToken(pikoToken),
		piko.WithURL("http://piko.example.com:8001"),
	)
	if err != nil {
		panic("connect: " + err.Error())
	}

	// Listen on endpoint 'my-endpoint'.
	ln, err := piko.Listen("my-endpoint")
	if err != nil {
		panic("listen: " + err.Error())
	}

	// Use the listener to accept HTTP requests.
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello world!")
	}
	if err := http.Serve(ln, http.HandlerFunc(handler)); err != nil {
		panic("http serve: " + err.Error())
	}
}
