package cluster

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"sync"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/testutil"
	"github.com/andydunstall/piko/server"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
)

type NodeV2 struct {
	server *server.Server

	rootCAPool *x509.CertPool

	ctx    context.Context
	cancel func()

	wg sync.WaitGroup
}

func NewNodeV2(opts ...Option) *NodeV2 {
	options := options{
		tls:    false,
		logger: log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	conf := config.Default()
	conf.Cluster.NodeID = cluster.GenerateNodeID()
	conf.Proxy.BindAddr = "127.0.0.1:0"
	conf.Upstream.BindAddr = "127.0.0.1:0"
	conf.Admin.BindAddr = "127.0.0.1:0"
	conf.Auth = options.authConfig

	// If TLS is enabled, generate a certificate and root CA then write to a
	// file.
	var rootCAPool *x509.CertPool
	if options.tls {
		conf.Proxy.TLS.Enabled = true
		conf.Upstream.TLS.Enabled = true
		conf.Admin.TLS.Enabled = true

		pool, cert, err := testutil.LocalTLSServerCert()
		if err != nil {
			panic("tls cert: " + err.Error())
		}
		rootCAPool = pool

		f, err := os.CreateTemp("", "piko")
		if err != nil {
			panic("create temp: " + err.Error())
		}

		for _, certBytes := range cert.Certificate {
			block := pem.Block{
				Type:  "CERTIFICATE",
				Bytes: certBytes,
			}
			if err := pem.Encode(f, &block); err != nil {
				panic("encode pem: " + err.Error())
			}
		}

		conf.Proxy.TLS.Cert = f.Name()
		conf.Upstream.TLS.Cert = f.Name()
		conf.Admin.TLS.Cert = f.Name()

		f, err = os.CreateTemp("", "piko")
		if err != nil {
			panic("create temp: " + err.Error())
		}

		keyBytes := x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey))

		block := pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: keyBytes,
		}

		if err := pem.Encode(f, &block); err != nil {
			panic("encode pem: " + err.Error())
		}

		conf.Proxy.TLS.Key = f.Name()
		conf.Upstream.TLS.Key = f.Name()
		conf.Admin.TLS.Key = f.Name()
	}

	server, err := server.NewServer(conf, options.logger)
	if err != nil {
		panic("server: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &NodeV2{
		server:     server,
		rootCAPool: rootCAPool,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (n *NodeV2) ProxyAddr() string {
	return n.server.Config().Proxy.AdvertiseAddr
}

func (n *NodeV2) UpstreamAddr() string {
	return n.server.Config().Upstream.AdvertiseAddr
}

func (n *NodeV2) AdminAddr() string {
	return n.server.Config().Admin.AdvertiseAddr
}

func (n *NodeV2) RootCAPool() *x509.CertPool {
	return n.rootCAPool
}

func (n *NodeV2) Start() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.server.Run(n.ctx); err != nil {
			panic("server: " + err.Error())
		}
	}()
}

func (n *NodeV2) Stop() {
	n.cancel()
	n.wg.Wait()
}
