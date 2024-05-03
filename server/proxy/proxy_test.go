package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/pkg/status"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

type fakeStream struct {
	rpcHandler func(rpcType rpc.Type, req []byte) ([]byte, error)
}

func (s *fakeStream) Addr() string {
	return ""
}

func (s *fakeStream) RPC(_ context.Context, rpcType rpc.Type, req []byte) ([]byte, error) {
	return s.rpcHandler(rpcType, req)
}

func (s *fakeStream) Monitor(
	_ context.Context,
	_ time.Duration,
	_ time.Duration,
) error {
	return nil
}

func (s *fakeStream) Close() error {
	return nil
}

func TestProxy_Request(t *testing.T) {
	t.Run("local endpoint ok", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{
			ID:     "local",
			Status: netmap.NodeStatusActive,
		}, log.NewNopLogger())
		proxy := NewProxy(networkMap, nil, log.NewNopLogger())

		rpcHandler := func(rpcType rpc.Type, req []byte) ([]byte, error) {
			assert.Equal(t, rpc.TypeProxyHTTP, rpcType)

			var protoReq api.ProxyHttpReq
			assert.NoError(t, proto.Unmarshal(req, &protoReq))

			httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(protoReq.HttpReq)))
			assert.NoError(t, err)
			assert.Equal(t, "/foo", httpReq.URL.Path)

			header := make(http.Header)
			header.Add("h1", "v1")
			header.Add("h2", "v2")
			header.Add("h3", "v3")
			body := bytes.NewReader([]byte("foo"))
			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(body),
			}
			var buffer bytes.Buffer
			assert.NoError(t, httpResp.Write(&buffer))

			protoResp := &api.ProxyHttpResp{
				HttpResp: buffer.Bytes(),
			}
			payload, err := proto.Marshal(protoResp)
			assert.NoError(t, err)

			return payload, nil
		}
		stream := &fakeStream{rpcHandler: rpcHandler}

		proxy.AddUpstream("my-endpoint", stream)

		resp, err := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com:8000",
		})
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "v1", resp.Header.Get("h1"))
		assert.Equal(t, "v2", resp.Header.Get("h2"))
		assert.Equal(t, "v3", resp.Header.Get("h3"))

		buf := new(bytes.Buffer)
		//nolint
		buf.ReadFrom(resp.Body)
		assert.Equal(t, []byte("foo"), buf.Bytes())
	})

	t.Run("endpoint unreachable", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{
			ID:     "local",
			Status: netmap.NodeStatusActive,
		}, log.NewNopLogger())
		proxy := NewProxy(networkMap, nil, log.NewNopLogger())

		rpcHandler := func(rpcType rpc.Type, req []byte) ([]byte, error) {
			return nil, fmt.Errorf("unreachable")
		}
		stream := &fakeStream{rpcHandler: rpcHandler}

		proxy.AddUpstream("my-endpoint", stream)

		_, err := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com:8000",
		})

		var errorInfo *status.ErrorInfo
		assert.Error(t, err)
		assert.True(t, errors.As(err, &errorInfo))
		assert.Equal(t, http.StatusServiceUnavailable, errorInfo.StatusCode)
		assert.Equal(t, "endpoint unreachable", errorInfo.Message)
	})

	t.Run("endpoint timeout", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{
			ID:     "local",
			Status: netmap.NodeStatusActive,
		}, log.NewNopLogger())
		proxy := NewProxy(networkMap, nil, log.NewNopLogger())

		rpcHandler := func(rpcType rpc.Type, req []byte) ([]byte, error) {
			return nil, fmt.Errorf("unreachable: %w", context.DeadlineExceeded)
		}
		stream := &fakeStream{rpcHandler: rpcHandler}

		proxy.AddUpstream("my-endpoint", stream)

		_, err := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com:8000",
		})

		var errorInfo *status.ErrorInfo
		assert.Error(t, err)
		assert.True(t, errors.As(err, &errorInfo))
		assert.Equal(t, http.StatusGatewayTimeout, errorInfo.StatusCode)
		assert.Equal(t, "endpoint timeout", errorInfo.Message)
	})

	t.Run("endpoint not found", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{
			ID:     "local",
			Status: netmap.NodeStatusActive,
		}, log.NewNopLogger())
		proxy := NewProxy(networkMap, nil, log.NewNopLogger())

		_, err := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com:8000",
		})

		var errorInfo *status.ErrorInfo
		assert.Error(t, err)
		assert.True(t, errors.As(err, &errorInfo))
		assert.Equal(t, http.StatusServiceUnavailable, errorInfo.StatusCode)
		assert.Equal(t, "endpoint not found", errorInfo.Message)
	})

	t.Run("missing endpoint id", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{
			ID:     "local",
			Status: netmap.NodeStatusActive,
		}, log.NewNopLogger())
		proxy := NewProxy(networkMap, nil, log.NewNopLogger())

		_, err := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "localhost:9000",
		})

		var errorInfo *status.ErrorInfo
		assert.Error(t, err)
		assert.True(t, errors.As(err, &errorInfo))
		assert.Equal(t, http.StatusServiceUnavailable, errorInfo.StatusCode)
		assert.Equal(t, "missing endpoint id", errorInfo.Message)
	})
}

func TestProxy_ParseEndpointID(t *testing.T) {
	t.Run("host header", func(t *testing.T) {
		endpointID := parseEndpointID(&http.Request{
			Host: "my-endpoint.pico.com:9000",
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("x-pico-endpoint header", func(t *testing.T) {
		header := make(http.Header)
		header.Add("x-pico-endpoint", "my-endpoint")
		endpointID := parseEndpointID(&http.Request{
			Host:   "localhost:9000",
			Header: header,
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("no endpoint", func(t *testing.T) {
		endpointID := parseEndpointID(&http.Request{
			Host: "localhost:9000",
		})
		assert.Equal(t, "", endpointID)
	})
}
