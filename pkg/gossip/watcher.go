package gossip

// Watcher is used to receive notifications when the known remote node state
// changes.
//
// The implementations of Watcher must not block. Watcher is also called with
// the state mutex held so should not call back to Gossip.
type Watcher interface {
	// OnJoin notifies that a new node joined the cluster.
	OnJoin(nodeID string)

	// OnLeave notifies that a node gracefully left the cluster.
	OnLeave(nodeID string)

	// OnReachable notifies that a node that was considered unreachable has
	// recovered.
	OnReachable(nodeID string)

	// OnUnreachable notifies that a node is considered unreachable.
	OnUnreachable(nodeID string)

	// OnUpsertKey notifies that a nodes state key has been updated.
	OnUpsertKey(nodeID, key, value string)

	// OnDeleteKey notifies that a nodes state key has been deleted.
	OnDeleteKey(nodeID, key string)

	// OnExpired notifies that a nodes state has expired and been removed.
	OnExpired(nodeID string)
}

type nopWatcher struct {
}

func newNopWatcher() *nopWatcher {
	return &nopWatcher{}
}

func (w *nopWatcher) OnJoin(_ string) {}

func (w *nopWatcher) OnLeave(_ string) {}

func (w *nopWatcher) OnReachable(_ string) {}

func (w *nopWatcher) OnUnreachable(_ string) {}

func (w *nopWatcher) OnUpsertKey(_, _, _ string) {}

func (w *nopWatcher) OnDeleteKey(_, _ string) {}

func (w *nopWatcher) OnExpired(_ string) {}

var _ Watcher = &nopWatcher{}
