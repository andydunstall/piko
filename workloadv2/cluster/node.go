package cluster

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/testutil"
	"github.com/andydunstall/piko/server"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
)

type Node struct {
	server *server.Server

	rootCAPool *x509.CertPool
}

func NewNode(opts ...Option) *Node {
	options := options{
		logger: log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	conf := config.Default()
	conf.Cluster.NodeID = cluster.GenerateNodeID()
	conf.Cluster.Join = options.join
	conf.Proxy.BindAddr = "127.0.0.1:0"
	conf.Upstream.BindAddr = "127.0.0.1:0"
	conf.Admin.BindAddr = "127.0.0.1:0"
	conf.Gossip.BindAddr = "127.0.0.1:0"
	conf.Gossip.Interval = time.Millisecond * 10
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

	server, err := server.NewServer(
		conf,
		options.logger.With(zap.String("node", conf.Cluster.NodeID)),
	)
	if err != nil {
		panic("server: " + err.Error())
	}

	return &Node{
		server:     server,
		rootCAPool: rootCAPool,
	}
}

func (n *Node) ProxyAddr() string {
	return n.server.Config().Proxy.AdvertiseAddr
}

func (n *Node) UpstreamAddr() string {
	return n.server.Config().Upstream.AdvertiseAddr
}

func (n *Node) AdminAddr() string {
	return n.server.Config().Admin.AdvertiseAddr
}

func (n *Node) GossipAddr() string {
	return n.server.Config().Gossip.AdvertiseAddr
}

func (n *Node) ClusterState() *cluster.State {
	return n.server.ClusterState()
}

func (n *Node) RootCAPool() *x509.CertPool {
	return n.rootCAPool
}

func (n *Node) Start() {
	if err := n.server.Start(); err != nil {
		panic("start node: " + err.Error())
	}
}

func (n *Node) Stop() {
	n.server.Shutdown()
}
