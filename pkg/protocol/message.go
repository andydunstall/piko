package protocol

import "golang.ngrok.com/muxado/v2"

type RPCType muxado.StreamType

const (
	RPCTypeListen RPCType = iota + 1
	RPCTypeProxy
)

type ListenRequest struct {
	EndpointID string `json:"endpoint_id"`
}

type ListenResponse struct {
	EndpointID string `json:"endpoint_id"`
}

type ProxyHeader struct {
	EndpointID string `json:"endpoint_id"`
}
