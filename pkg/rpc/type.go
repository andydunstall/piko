package rpc

// Type is an identifier for the RPC request/response type.
type Type uint16

const (
	TypeHeartbeat = iota + 1
)

func (t *Type) String() string {
	switch *t {
	case TypeHeartbeat:
		return "heartbeat"
	default:
		return "unknown"
	}
}
