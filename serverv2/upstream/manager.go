package upstream

import (
	"fmt"
	"net"
	"sync"

	"github.com/andydunstall/piko/pkg/protocol"
	"golang.ngrok.com/muxado/v2"
)

// Manager manages the set of local upsteam services.
//
// TODO(andydunstall): Only supports a single upstream.
type Manager struct {
	upstream *Upstream

	mu sync.Mutex
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Add(upstream *Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upstream = upstream
}

func (m *Manager) Remove(_ *Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.upstream = nil
}

func (m *Manager) Dial() (net.Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.upstream == nil {
		return nil, fmt.Errorf("not found")
	}

	stream, err := m.upstream.Sess.OpenTypedStream(
		muxado.StreamType(protocol.RPCTypeProxy),
	)
	if err != nil {
		return nil, err
	}

	return stream, nil
}
