package conn

import (
	"context"
	"fmt"
	"io"

	"github.com/gorilla/websocket"
)

type WebsocketConn struct {
	wsConn *websocket.Conn
}

func NewWebsocketConn(wsConn *websocket.Conn) *WebsocketConn {
	return &WebsocketConn{
		wsConn: wsConn,
	}
}

func DialWebsocket(ctx context.Context, url string) (*WebsocketConn, error) {
	wsConn, _, err := websocket.DefaultDialer.DialContext(
		ctx, url, nil,
	)
	if err != nil {
		return nil, err
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
