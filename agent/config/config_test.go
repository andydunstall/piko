package config

import (
	"net/url"
	"os"
	"testing"
	"time"

	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
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

func TestConfig_LoadYAML(t *testing.T) {
	yaml := `
listeners:
  - endpoint_id: '123'
    addr: 'http://localhost:9208'
    timeout: 7m
    protocol: http
    http_client:
      keep_alive_timeout: 10m
connect:
  url: 'http://localhost:8001'
  timeout: 30s
  token: cyz
server:
  enabled: true
  bind_addr: ':5201'
log:
  level: info
`

	f, err := os.CreateTemp("", "piko-agent")
	assert.NoError(t, err)

	_, err = f.WriteString(yaml)
	assert.NoError(t, err)

	var loadedConf Config
	assert.NoError(t, pikoconfig.Load(&loadedConf, f.Name(), false))

	expectedConf := Config{
		Listeners: []ListenerConfig{{
			EndpointID: "123",
			Addr:       "http://localhost:9208",
			Protocol:   "http",
			AccessLog: log.AccessLogConfig{
				Disable: false,
				RequestHeaders: log.AccessLogHeaderConfig{
					AllowList: nil,
					BlockList: nil,
				},
				ResponseHeaders: log.AccessLogHeaderConfig{
					AllowList: nil,
					BlockList: nil,
				},
			},
			Timeout: 7 * time.Minute,
			HttpClient: ListenerHttpClientConfig{
				KeepAliveTimeout:      10 * time.Minute,
				MaxIdleConnections:    0,
				IdleConnectionTimeout: 0,
				DisableCompression:    false,
			},
			TLS: TLSConfig{
				Cert:               "",
				Key:                "",
				RootCAs:            "",
				InsecureSkipVerify: false,
			},
		}},
		Connect: ConnectConfig{
			URL:     "http://localhost:8001",
			Timeout: 30 * time.Second,
			Token:   "cyz",
		},
		Server: ServerConfig{
			Enabled:  true,
			BindAddr: ":5201",
		},
		Log: log.Config{
			Level:      "info",
			Subsystems: nil,
		},
		GracePeriod: 0,
	}

	assert.Equal(t, expectedConf, loadedConf)
}
