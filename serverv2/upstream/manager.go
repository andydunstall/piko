package upstream

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/andydunstall/piko/pkg/protocol"
	"golang.ngrok.com/muxado/v2"
)

// Manager manages the set of local upsteam services.
type Manager struct {
	upstreams map[string][]*Upstream

	mu sync.Mutex
}

func NewManager() *Manager {
	return &Manager{
		upstreams: make(map[string][]*Upstream),
	}
}

func (m *Manager) Add(upstream *Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upstreams[upstream.EndpointID] = append(
		m.upstreams[upstream.EndpointID], upstream,
	)
}

func (m *Manager) Dial(endpointID string) (net.Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	upstreams, ok := m.upstreams[endpointID]
	if !ok || len(upstreams) == 0 {
		return nil, fmt.Errorf("not found")
	}

	stream, err := upstreams[0].Sess.OpenTypedStream(
		muxado.StreamType(protocol.RPCTypeProxy),
	)
	if err != nil {
		return nil, err
	}

	header := protocol.ProxyHeader{
		EndpointID: endpointID,
	}
	if err := json.NewEncoder(stream).Encode(header); err != nil {
		return nil, fmt.Errorf("write proxy header: %w", err)
	}

	return stream, nil
}
