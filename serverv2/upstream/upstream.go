package upstream

import (
	"net"

	"golang.ngrok.com/muxado/v2"
)

type Upstream interface {
	EndpointID() string
	Dial() (net.Conn, error)
}

type TCPUpstream struct {
	endpointID string
	addr       string
}

func NewTCPUpstream(endpointID, addr string) *TCPUpstream {
	return &TCPUpstream{
		endpointID: endpointID,
		addr:       addr,
	}
}

func (u *TCPUpstream) EndpointID() string {
	return u.endpointID
}

func (u *TCPUpstream) Dial() (net.Conn, error) {
	return net.Dial("tcp", u.addr)
}

type MuxUpstream struct {
	endpointID string
	sess       muxado.TypedStreamSession
}

func NewMuxUpstream(endpointID string, sess muxado.TypedStreamSession) *MuxUpstream {
	return &MuxUpstream{
		endpointID: endpointID,
		sess:       sess,
	}
}

func (u *MuxUpstream) EndpointID() string {
	return u.endpointID
}

func (u *MuxUpstream) Dial() (net.Conn, error) {
	return u.sess.OpenTypedStream(0)
}
