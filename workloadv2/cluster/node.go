package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
	"go.uber.org/zap"
)

type Node struct {
	server *server.Server

	ctx    context.Context
	cancel func()

	wg sync.WaitGroup
}

func NewNode(join []string, logger log.Logger) *Node {
	conf := config.Default()
	conf.Cluster.NodeID = cluster.GenerateNodeID()
	conf.Cluster.Join = join
	conf.Proxy.BindAddr = "127.0.0.1:0"
	conf.Upstream.BindAddr = "127.0.0.1:0"
	conf.Admin.BindAddr = "127.0.0.1:0"
	conf.Gossip.BindAddr = "127.0.0.1:0"
	conf.Gossip.Interval = time.Millisecond * 10

	server, err := server.NewServer(
		conf,
		logger.With(zap.String("node", conf.Cluster.NodeID)),
	)
	if err != nil {
		panic("server: " + err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Node{
		server: server,
		ctx:    ctx,
		cancel: cancel,
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

func (n *Node) Start() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.server.Run(n.ctx); err != nil {
			panic("server: " + err.Error())
		}
	}()
}

func (n *Node) Stop() {
	n.cancel()
	n.wg.Wait()
}
