package proxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/stretchr/testify/assert"
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

func TestRPCStream(t *testing.T) {
	rpcHandler := func(rpcType rpc.Type, req []byte) ([]byte, error) {
		assert.Equal(t, rpc.TypeProxyHTTP, rpcType)

		httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
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

		return buffer.Bytes(), nil
	}
	stream := &fakeStream{rpcHandler: rpcHandler}

	conn := NewRPCConn("my-endpoint", stream)

	resp, err := conn.Request(context.TODO(), &http.Request{
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
}
