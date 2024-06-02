package upstream

import "golang.ngrok.com/muxado/v2"

type Upstream struct {
	EndpointID string
	Sess       muxado.TypedStreamSession
}
