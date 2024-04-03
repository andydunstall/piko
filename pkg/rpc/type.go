package rpc

// Type is an identifier for the RPC request/response type.
type Type uint16

const (
	// TypeHeartbeat sends health checks between peers.
	TypeHeartbeat = iota + 1
	// TypeProxyHTTP sends a HTTP request and response between the Pico server
	// and an upstream listener.
	TypeProxyHTTP
)

func (t *Type) String() string {
	switch *t {
	case TypeHeartbeat:
		return "heartbeat"
	case TypeProxyHTTP:
		return "proxy-http"
	default:
		return "unknown"
	}
}
