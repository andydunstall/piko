package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests the default configuration is valid.
func TestConfig_Default(t *testing.T) {
	conf := Default()
	assert.NoError(t, conf.Validate())
}

func TestListenerConfig_URL(t *testing.T) {
	tests := []struct {
		addr string
		url  *url.URL
		ok   bool
	}{
		{
			addr: "8080",
			url: &url.URL{
				Scheme: "http",
				Host:   "localhost:8080",
			},
		},
		{
			addr: "1.2.3.4:8080",
			url: &url.URL{
				Scheme: "http",
				Host:   "1.2.3.4:8080",
			},
		},
		{
			addr: "https://1.2.3.4:8080",
			url: &url.URL{
				Scheme: "https",
				Host:   "1.2.3.4:8080",
			},
		},
		{
			addr: "https://google.com:443",
			url: &url.URL{
				Scheme: "https",
				Host:   "google.com:443",
			},
		},
		{
			addr: "https://google.com",
			url: &url.URL{
				Scheme: "https",
				Host:   "google.com",
			},
		},
		{
			addr: "invalid",
			ok:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.addr, func(t *testing.T) {
			conf := &ListenerConfig{Addr: tt.addr}
			u, ok := conf.URL()
			if !ok {
				assert.Equal(t, tt.ok, ok)
				return
			}
			assert.Equal(t, tt.url, u)
		})
	}
}
