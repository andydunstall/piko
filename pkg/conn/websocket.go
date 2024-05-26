package conn

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
)

// retryableStatusCodes contains a set of HTTP status codes that should be
// retried.
var retryableStatusCodes = map[int]struct{}{
	http.StatusRequestTimeout:      {},
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
}

type WebsocketConn struct {
	wsConn *websocket.Conn
}

func NewWebsocketConn(wsConn *websocket.Conn) *WebsocketConn {
	return &WebsocketConn{
		wsConn: wsConn,
	}
}

func DialWebsocket(ctx context.Context, url string, token string) (*WebsocketConn, error) {
	header := make(http.Header)
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	wsConn, resp, err := websocket.DefaultDialer.DialContext(
		ctx, url, header,
	)
	if err != nil {
		if resp != nil {
			if _, ok := retryableStatusCodes[resp.StatusCode]; ok {
				return nil, &RetryableError{err}
			}
			return nil, fmt.Errorf("%d: %w", resp.StatusCode, err)
		}
		return nil, &RetryableError{err}
	}
	return NewWebsocketConn(wsConn), nil
}

func (t *WebsocketConn) ReadMessage() ([]byte, error) {
	mt, message, err := t.wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", mt)
	}
	return message, nil
}

func (t *WebsocketConn) NextReader() (io.Reader, error) {
	mt, r, err := t.wsConn.NextReader()
	if err != nil {
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", mt)
	}
	return r, nil
}

func (t *WebsocketConn) WriteMessage(b []byte) error {
	return t.wsConn.WriteMessage(websocket.BinaryMessage, b)
}

func (t *WebsocketConn) NextWriter() (io.WriteCloser, error) {
	return t.wsConn.NextWriter(websocket.BinaryMessage)
}

func (t *WebsocketConn) Addr() string {
	return t.wsConn.RemoteAddr().String()
}

func (t *WebsocketConn) Close() error {
	return t.wsConn.Close()
}

var _ Conn = &WebsocketConn{}
