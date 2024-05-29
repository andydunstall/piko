package piko_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	piko "github.com/andydunstall/piko/client"
)

func Example() {
	pikoAPIKey := os.Getenv("PIKO_API_KEY")

	// Connect to the Piko server.
	piko, err := piko.Connect(
		context.Background(),
		piko.WithAPIKey(pikoAPIKey),
		piko.WithURL("http://piko.example.com:8001"),
	)
	if err != nil {
		panic("connect: " + err.Error())
	}

	// Listen on endpoint 'my-endpoint'.
	ln, err := piko.Listen(context.Background(), "my-endpoint")
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
