//go:build system

package tests

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server"
	statusclient "github.com/andydunstall/pico/status/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster(t *testing.T) {
	t.Run("discover", func(t *testing.T) {
		var wg sync.WaitGroup

		server1Conf := defaultServerConfig()
		server1, err := server.NewServer(server1Conf, log.NewNopLogger())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, server1.Run(ctx))
		}()

		server2Conf := defaultServerConfig()
		server2Conf.Cluster.Join = []string{server1Conf.Gossip.AdvertiseAddr}
		server2, err := server.NewServer(server2Conf, log.NewNopLogger())
		require.NoError(t, err)

		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, server2.Run(ctx))
		}()

		// Wait for each server to discover the other.
		for _, addr := range []string{
			server1Conf.Admin.AdvertiseAddr,
			server2Conf.Admin.AdvertiseAddr,
		} {
			for {
				statusClient := statusclient.NewClient(&url.URL{
					Scheme: "http",
					Host:   addr,
				}, "")
				nodes, err := statusClient.ClusterNodes()
				assert.NoError(t, err)

				if len(nodes) < 2 {
					<-time.After(time.Millisecond * 10)
					continue
				}
				break
			}
		}

		cancel()
		wg.Wait()
	})
}
