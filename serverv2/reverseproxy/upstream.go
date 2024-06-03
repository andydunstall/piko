package reverseproxy

import (
	"net"
)

// UpstreamPool contains the connected upstreams.
type UpstreamPool interface {
	Dial(endpointID string) (net.Conn, error)
}
