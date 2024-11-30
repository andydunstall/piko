package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/pkg/auth"
	"github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/gossip"
	"github.com/andydunstall/piko/pkg/log"
)

// Tests the default configuration is valid (not including node ID).
func TestConfig_Default(t *testing.T) {
	conf := Default()
	conf.Cluster.NodeID = "my-node"
	assert.NoError(t, conf.Validate())
}

// Tests loading the server configuration from YAML.
func TestConfig_LoadYAML(t *testing.T) {
	yaml := `
proxy:
  bind_addr: 10.15.104.25:8000
  advertise_addr: 1.2.3.4:8000
  timeout: 20s
  access_log: true

  http:
    read_timeout: 5s
    read_header_timeout: 5s
    write_timeout: 5s
    idle_timeout: 2s
    max_header_bytes: 2097152

  auth:
    hmac_secret_key: hmac-secret-key
    rsa_public_key: rsa-public-key
    ecdsa_public_key: ecdsa-public-key
    audience: my-audience
    issuer: my-issuer

  tls:
    cert: /piko/cert.pem
    key: /piko/key.pem

upstream:
  bind_addr: 10.15.104.25:8001
  advertise_addr: 1.2.3.4:8001

  auth:
    hmac_secret_key: hmac-secret-key
    rsa_public_key: rsa-public-key
    ecdsa_public_key: ecdsa-public-key
    audience: my-audience
    issuer: my-issuer

  tls:
    cert: /piko/cert.pem
    key: /piko/key.pem

admin:
  bind_addr: 10.15.104.25:8002
  advertise_addr: 1.2.3.4:8002

  auth:
    hmac_secret_key: hmac-secret-key
    rsa_public_key: rsa-public-key
    ecdsa_public_key: ecdsa-public-key
    audience: my-audience
    issuer: my-issuer

  tls:
    cert: /piko/cert.pem
    key: /piko/key.pem

cluster:
  node_id: "my-node"
  join:
    - 10.26.104.12:8003
    - 10.26.104.73:8003
    - 10.26.104.28:8003
  join_timeout: 2m
  abort_if_join_fails: true

  gossip:
    bind_addr: 10.15.104.25:8003
    advertise_addr: 1.2.3.4:8003
    interval: 100ms
    max_packet_size: 1400

usage:
  disable: true

log:
  level: info
  subsystems:
    - foo
    - bar

grace_period: 2m
`

	f, err := os.CreateTemp("", "piko")
	assert.NoError(t, err)

	_, err = f.WriteString(yaml)
	assert.NoError(t, err)

	var loadedConf Config

	assert.NoError(t, config.Load(&loadedConf, f.Name(), false))

	expectedConf := Config{
		Proxy: ProxyConfig{
			BindAddr:      "10.15.104.25:8000",
			AdvertiseAddr: "1.2.3.4:8000",
			Timeout:       time.Second * 20,
			AccessLog:     true,
			HTTP: HTTPConfig{
				ReadTimeout:       time.Second * 5,
				ReadHeaderTimeout: time.Second * 5,
				WriteTimeout:      time.Second * 5,
				IdleTimeout:       time.Second * 2,
				MaxHeaderBytes:    2097152,
			},
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Upstream: UpstreamConfig{
			BindAddr:      "10.15.104.25:8001",
			AdvertiseAddr: "1.2.3.4:8001",
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Admin: AdminConfig{
			BindAddr:      "10.15.104.25:8002",
			AdvertiseAddr: "1.2.3.4:8002",
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Cluster: ClusterConfig{
			NodeID: "my-node",
			Join: []string{
				"10.26.104.12:8003",
				"10.26.104.73:8003",
				"10.26.104.28:8003",
			},
			JoinTimeout:      2 * time.Minute,
			AbortIfJoinFails: true,
			Gossip: gossip.Config{
				BindAddr:      "10.15.104.25:8003",
				AdvertiseAddr: "1.2.3.4:8003",
				Interval:      time.Millisecond * 100,
				MaxPacketSize: 1400,
			},
		},
		Usage: UsageConfig{
			Disable: true,
		},
		Log: log.Config{
			Level: "info",
			Subsystems: []string{
				"foo",
				"bar",
			},
		},
		GracePeriod: 2 * time.Minute,
	}
	assert.Equal(t, expectedConf, loadedConf)
}

// Tests loading the server configuration from flags.
func TestConfig_LoadFlags(t *testing.T) {
	args := []string{
		"--proxy.bind-addr", "10.15.104.25:8000",
		"--proxy.advertise-addr", "1.2.3.4:8000",
		"--proxy.timeout", "20s",
		"--proxy.access-log",
		"--proxy.http.read-timeout", "5s",
		"--proxy.http.read-header-timeout", "5s",
		"--proxy.http.write-timeout", "5s",
		"--proxy.http.idle-timeout", "2s",
		"--proxy.http.max-header-bytes", "2097152",
		"--proxy.auth.hmac-secret-key", "hmac-secret-key",
		"--proxy.auth.rsa-public-key", "rsa-public-key",
		"--proxy.auth.ecdsa-public-key", "ecdsa-public-key",
		"--proxy.auth.audience", "my-audience",
		"--proxy.auth.issuer", "my-issuer",
		"--proxy.tls.cert", "/piko/cert.pem",
		"--proxy.tls.key", "/piko/key.pem",
		"--upstream.bind-addr", "10.15.104.25:8001",
		"--upstream.advertise-addr", "1.2.3.4:8001",
		"--upstream.auth.hmac-secret-key", "hmac-secret-key",
		"--upstream.auth.rsa-public-key", "rsa-public-key",
		"--upstream.auth.ecdsa-public-key", "ecdsa-public-key",
		"--upstream.auth.audience", "my-audience",
		"--upstream.auth.issuer", "my-issuer",
		"--upstream.tls.cert", "/piko/cert.pem",
		"--upstream.tls.key", "/piko/key.pem",
		"--admin.bind-addr", "10.15.104.25:8002",
		"--admin.advertise-addr", "1.2.3.4:8002",
		"--admin.auth.hmac-secret-key", "hmac-secret-key",
		"--admin.auth.rsa-public-key", "rsa-public-key",
		"--admin.auth.ecdsa-public-key", "ecdsa-public-key",
		"--admin.auth.audience", "my-audience",
		"--admin.auth.issuer", "my-issuer",
		"--admin.tls.cert", "/piko/cert.pem",
		"--admin.tls.key", "/piko/key.pem",
		"--cluster.node-id", "my-node",
		"--cluster.join", "10.26.104.12:8003,10.26.104.73:8003,10.26.104.28:8003",
		"--cluster.join-timeout", "2m",
		"--cluster.abort-if-join-fails",
		"--cluster.gossip.bind-addr", "10.15.104.25:8003",
		"--cluster.gossip.advertise-addr", "1.2.3.4:8003",
		"--cluster.gossip.interval", "100ms",
		"--cluster.gossip.max-packet-size", "1400",
		"--usage.disable",
		"--log.level", "info",
		"--log.subsystems", "foo,bar",
		"--grace-period", "2m",
	}

	fs := pflag.NewFlagSet("", pflag.PanicOnError)

	var loadedConf Config
	loadedConf.RegisterFlags(fs)

	assert.NoError(t, fs.Parse(args))

	expectedConf := Config{
		Proxy: ProxyConfig{
			BindAddr:      "10.15.104.25:8000",
			AdvertiseAddr: "1.2.3.4:8000",
			Timeout:       time.Second * 20,
			AccessLog:     true,
			HTTP: HTTPConfig{
				ReadTimeout:       time.Second * 5,
				ReadHeaderTimeout: time.Second * 5,
				WriteTimeout:      time.Second * 5,
				IdleTimeout:       time.Second * 2,
				MaxHeaderBytes:    2097152,
			},
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Upstream: UpstreamConfig{
			BindAddr:      "10.15.104.25:8001",
			AdvertiseAddr: "1.2.3.4:8001",
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Admin: AdminConfig{
			BindAddr:      "10.15.104.25:8002",
			AdvertiseAddr: "1.2.3.4:8002",
			Auth: auth.Config{
				HMACSecretKey:  "hmac-secret-key",
				RSAPublicKey:   "rsa-public-key",
				ECDSAPublicKey: "ecdsa-public-key",
				Audience:       "my-audience",
				Issuer:         "my-issuer",
			},
			TLS: TLSConfig{
				Cert: "/piko/cert.pem",
				Key:  "/piko/key.pem",
			},
		},
		Cluster: ClusterConfig{
			NodeID: "my-node",
			Join: []string{
				"10.26.104.12:8003",
				"10.26.104.73:8003",
				"10.26.104.28:8003",
			},
			JoinTimeout:      2 * time.Minute,
			AbortIfJoinFails: true,
			Gossip: gossip.Config{
				BindAddr:      "10.15.104.25:8003",
				AdvertiseAddr: "1.2.3.4:8003",
				Interval:      time.Millisecond * 100,
				MaxPacketSize: 1400,
			},
		},
		Usage: UsageConfig{
			Disable: true,
		},
		Log: log.Config{
			Level: "info",
			Subsystems: []string{
				"foo",
				"bar",
			},
		},
		GracePeriod: 2 * time.Minute,
	}
	assert.Equal(t, expectedConf, loadedConf)
}
