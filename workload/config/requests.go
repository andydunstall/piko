package config

import (
	"fmt"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type RequestsConfig struct {
	Clients int `json:"clients" yaml:"clients"`

	// Rate is the number of requests per second per client.
	Rate int `json:"rate" yaml:"rate"`

	// Endpoints is the number of available endpoint IDs.
	Endpoints int `json:"endpoints" yaml:"endpoints"`

	// RequestSize is the size of each request.
	RequestSize int `json:"request_size" yaml:"request_size"`

	Server ServerConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func (c *RequestsConfig) Validate() error {
	if c.Clients == 0 {
		return fmt.Errorf("missing clients")
	}
	if c.Rate == 0 {
		return fmt.Errorf("missing rate")
	}
	if c.Endpoints == 0 {
		return fmt.Errorf("missing endpoints")
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}

func (c *RequestsConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.IntVar(
		&c.Clients,
		"clients",
		50,
		`
The number of clients to run.`,
	)

	fs.IntVar(
		&c.Rate,
		"rate",
		10,
		`
The number of requests per second per client to send.`,
	)

	fs.IntVar(
		&c.Endpoints,
		"endpoints",
		100,
		`
The number of available endpoint IDs to send requests to.

On each request, the client selects a random endpoint ID from 0 to
'endpoints'.`,
	)

	fs.IntVar(
		&c.RequestSize,
		"request-size",
		1024,
		`
The size of each request. As the upstream echos the response body, the response
will have the same size.`,
	)

	fs.StringVar(
		&c.Server.URL,
		"server.url",
		"http://localhost:8000",
		`
Piko server proxy URL.`,
	)

	c.Log.RegisterFlags(fs)
}
