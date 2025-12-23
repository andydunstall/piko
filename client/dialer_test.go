package client_test

import (
	"context"

	piko "github.com/dragonflydb/piko/client"
)

// ExampleDialer opens a connection to endpoint 'my-endpoint' using Piko.
func ExampleDialer() {
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
