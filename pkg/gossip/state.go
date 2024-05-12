package gossip

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// leftKey is used to indicate a node left the cluster.
	leftKey = "_internal:left"

	// compactKey is used to indicate the version nodes can discard after a
	// compaction.
	compactKey = "_internal:compact"

	// nodeExpiry is the duration a left or unreachable node is stored until it
	// is is removed.
	nodeExpiry = time.Minute
)

// Entry represents a versioned key-value pair state.
type Entry struct {
	Key     string `json:"key" codec:"key"`
	Value   string `json:"value" codec:"value"`
	Version uint64 `json:"version" codec:"version"`

	// Internal indicates whether this is an internal entry.
	Internal bool `json:"internal" codec:"internal"`
	// Deleted indicates whether this entry represents a deleted key.
	Deleted bool `json:"deleted" codec:"deleted"`
}

// NodeMetadata contains the known metadata about the node.
type NodeMetadata struct {
	// ID is a unique identifier for the node.
	ID string `json:"id"`

	// Addr is the gossip address of the node.
	Addr string `json:"addr"`

	// Version is the latest known version of the node.
	Version uint64 `json:"version"`

	// Left indicates whether the node has left the cluster.
	Left bool `json:"left"`

	// Unreachable indicates whether the node is considered unreachable.
	Unreachable bool `json:"unreachable"`

	// Expiry contains the time the node state will expire. This is only set
	// if the node is considered left or unreachable until the expiry.
	Expiry time.Time
}

// NodeState contains the known state for the node.
type NodeState struct {
	NodeMetadata

	Entries []Entry
}

type digestEntry struct {
	ID      string `codec:"id"`
	Addr    string `codec:"addr"`
	Version uint64 `codec:"version"`
	Left    bool   `json:"left"`
}

type digest []digestEntry

type deltaEntry struct {
	ID      string  `codec:"id"`
	Addr    string  `codec:"addr"`
	Entries []Entry `codec:"entries"`
}

type delta []deltaEntry

type nodeState struct {
	NodeMetadata

	Entries map[string]Entry
}

func (s *nodeState) ToNodeState() *NodeState {
	var entries []Entry
	for _, entry := range s.Entries {
		entries = append(entries, entry)
	}
	// Sort by version.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version < entries[j].Version
	})
	return &NodeState{
		NodeMetadata: s.NodeMetadata,
		Entries:      entries,
	}
}

// clusterState contains the known state of each node in the cluster.
//
// The local state will always be up to date as it can only be updated locally.
// The state of remote nodes is eventually consistent and propagated via
// gossip.
type clusterState struct {
	localID string
	nodes   map[string]*nodeState

	// mu protects the above fields.
	mu sync.Mutex

	failureDetector failureDetector

	metrics *Metrics

	watcher Watcher
}

// newClusterState creates the cluster state with the local node.
func newClusterState(
	localID string,
	localAddr string,
	failureDetector failureDetector,
	metrics *Metrics,
	watcher Watcher,
) *clusterState {
	nodes := make(map[string]*nodeState)
	nodes[localID] = &nodeState{
		NodeMetadata: NodeMetadata{
			ID:      localID,
			Addr:    localAddr,
			Version: 0,
		},
		Entries: make(map[string]Entry),
	}

	return &clusterState{
		localID:         localID,
		nodes:           nodes,
		failureDetector: failureDetector,
		metrics:         metrics,
		watcher:         watcher,
	}
}

func (s *clusterState) Node(id string) (*NodeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[id]
	if !ok {
		return nil, false
	}
	return node.ToNodeState(), true
}

func (s *clusterState) LocalNodeMetadata() NodeMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.nodes[s.localID].NodeMetadata
}

func (s *clusterState) LocalNode() *NodeState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.nodes[s.localID].ToNodeState()
}

func (s *clusterState) Nodes() []NodeMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadata []NodeMetadata
	for _, node := range s.nodes {
		metadata = append(metadata, node.NodeMetadata)
	}
	return metadata
}

// LiveNodes returns the known remote nodes that are up and have not left.
func (s *clusterState) LiveNodes() []NodeMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadata []NodeMetadata
	for _, node := range s.nodes {
		if node.ID == s.localID {
			continue
		}
		if node.Unreachable || node.Left {
			continue
		}
		metadata = append(metadata, node.NodeMetadata)
	}
	return metadata
}

// UnreachableNodes returns the known remote nodes that are considered
// unreachable.
func (s *clusterState) UnreachableNodes() []NodeMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadata []NodeMetadata
	for _, node := range s.nodes {
		if node.ID == s.localID {
			continue
		}
		if node.Unreachable {
			metadata = append(metadata, node.NodeMetadata)
		}
	}
	return metadata
}

func (s *clusterState) UpsertLocal(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.nodes[s.localID]

	existing, ok := state.Entries[key]
	if ok {
		// If the entry is unchanged do nothing.
		if existing.Value == value {
			return
		}
	}

	state.Version++
	state.Entries[key] = Entry{
		Key:     key,
		Value:   value,
		Version: state.Version,
	}

	s.metricsUpsertEntry(state.ID, state.Entries[key], existing)
}

func (s *clusterState) DeleteLocal(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.nodes[s.localID]

	existing, ok := state.Entries[key]
	// If there is no entry for that key do nothing.
	if !ok {
		return
	}
	// If the entry is already deleted do nothing.
	if existing.Deleted {
		return
	}

	state.Version++

	state.Entries[key] = Entry{
		Key:      existing.Key,
		Value:    "",
		Version:  state.Version,
		Internal: existing.Internal,
		Deleted:  true,
	}

	s.metricsUpsertEntry(state.ID, state.Entries[key], existing)
}

// LeaveLocal updates the local node state to indicate the node has left the
// cluster.
func (s *clusterState) LeaveLocal() {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.nodes[s.localID]
	if state.Left {
		// Already left.
		return
	}

	state.Left = true

	state.Version++
	state.Entries[leftKey] = Entry{
		Key:      leftKey,
		Version:  state.Version,
		Internal: true,
	}

	s.metricsAddEntry(state.ID, state.Entries[leftKey])
}

// CompactLocal compacts the entries in the local node state to remove
// deleted keys if the number of deleted keys exceeds the given threshold.
//
// This will re-version all non-deleted keys, then add a special key to remove
// all entries prior to the reversioning.
func (s *clusterState) CompactLocal(threshold int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.nodes[s.localID]

	// Only compact if over the given threshold are deleted.
	deleted := 0
	for _, entry := range state.Entries {
		if entry.Deleted {
			deleted++
		}
	}
	if deleted < threshold {
		return
	}

	var entries []Entry
	for _, entry := range state.Entries {
		entries = append(entries, entry)
	}
	// Sort by version.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version < entries[j].Version
	})

	// compactVersion is last version we discarded.
	compactVersion := entries[len(entries)-1].Version

	// Clear existing entries.
	state.Entries = make(map[string]Entry)

	s.metrics.Entries.DeletePartialMatch(prometheus.Labels{
		"node_id": state.ID,
	})

	for _, entry := range entries {
		if entry.Deleted {
			// Discard deleted entries.
			continue
		}
		if entry.Internal && entry.Key == compactKey {
			// Discard overridden compaction entries.
			continue
		}

		state.Version++
		entry.Version = state.Version
		state.Entries[entry.Key] = entry

		s.metricsAddEntry(state.ID, entry)
	}

	state.Version++
	state.Entries[compactKey] = Entry{
		Key:      compactKey,
		Value:    strconv.FormatUint(compactVersion, 10),
		Version:  state.Version,
		Internal: true,
	}

	s.metricsAddEntry(state.ID, state.Entries[compactKey])
}

func (s *clusterState) Digest() digest {
	s.mu.Lock()
	defer s.mu.Unlock()

	var digest digest
	for _, state := range s.nodes {
		digest = append(digest, digestEntry{
			ID:      state.ID,
			Addr:    state.Addr,
			Version: state.Version,
			Left:    state.Left,
		})
	}
	return digest
}

// Delta returns a delta for the given digest. If fullDigest is true
// we assume the digest contains the full remote nodes known state so can
// include any nodes that is doesn't contain.
func (s *clusterState) Delta(digest digest, fullDigest bool) delta {
	s.mu.Lock()
	defer s.mu.Unlock()

	// digestNodes is the set of nodes in the digest.
	digestNodes := make(map[string]struct{})

	var delta delta
	for _, entry := range digest {
		digestNodes[entry.ID] = struct{}{}

		if _, ok := s.nodes[entry.ID]; !ok {
			// We have no state for this member.
			continue
		}

		deltaEntry := s.deltaEntry(entry.ID, entry.Version)
		// If we don't have any state on the member the sender doesn't,
		// don't include the member delta.
		if len(deltaEntry.Entries) > 0 {
			delta = append(delta, deltaEntry)
		}
	}

	// If we know we have a full digest from the client, we can add infer
	// that the client doesn't know about any nodes not included in their
	// digest.
	if fullDigest {
		for id := range s.nodes {
			if _, ok := digestNodes[id]; ok {
				continue
			}
			delta = append(delta, s.deltaEntry(id, 0))
		}
	}

	return delta
}

// LocalDelta returns a full delta for the local member.
func (s *clusterState) LocalDelta() delta {
	s.mu.Lock()
	defer s.mu.Unlock()

	return delta{s.deltaEntry(s.localID, 0)}
}

// ApplyDigest discovers any nodes we don't yet know about from the given
// digest and adds them to our local state with a version of 0.
func (s *clusterState) ApplyDigest(digest digest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range digest {
		// If we already know about the member theres nothing to do.
		if _, ok := s.nodes[entry.ID]; ok {
			continue
		}
		// If we a node has left the cluster and we don't know about it
		// already, then ignore it. Otherwise nodes will keep being
		// re-discovered after they left.
		if entry.Left {
			continue
		}

		s.nodes[entry.ID] = &nodeState{
			NodeMetadata: NodeMetadata{
				ID:      entry.ID,
				Addr:    entry.Addr,
				Version: 0,
			},
			Entries: make(map[string]Entry),
		}

		s.watcher.OnJoin(entry.ID)
	}
}

// ApplyDelta updates the state of remote nodes given the delta state.
func (s *clusterState) ApplyDelta(delta delta) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range delta {
		s.applyDeltaEntry(entry)
	}
}

func (s *clusterState) deltaEntry(nodeID string, fromVersion uint64) deltaEntry {
	state := s.nodes[nodeID]

	deltaEntry := deltaEntry{
		ID:   state.ID,
		Addr: state.Addr,
	}

	for _, entry := range state.Entries {
		if entry.Version <= fromVersion {
			continue
		}

		deltaEntry.Entries = append(deltaEntry.Entries, entry)
	}

	// Sort by version.
	sort.Slice(deltaEntry.Entries, func(i, j int) bool {
		return deltaEntry.Entries[i].Version < deltaEntry.Entries[j].Version
	})

	return deltaEntry
}

func (s *clusterState) applyDeltaEntry(entry deltaEntry) {
	if entry.ID == s.localID {
		// Discard updates about local node.
		return
	}

	state, ok := s.nodes[entry.ID]
	if !ok {
		s.nodes[entry.ID] = &nodeState{
			NodeMetadata: NodeMetadata{
				ID:   entry.ID,
				Addr: entry.Addr,
			},
			Entries: make(map[string]Entry),
		}
		state = s.nodes[entry.ID]

		s.watcher.OnJoin(entry.ID)
	}

	for _, e := range entry.Entries {
		// Discard old versions.
		if e.Version <= state.Version {
			continue
		}

		existing := state.Entries[e.Key]
		state.Entries[e.Key] = e
		state.Version = e.Version

		s.metricsUpsertEntry(state.ID, e, existing)

		if e.Internal {
			if e.Key == leftKey {
				state.Left = true
				state.Expiry = time.Now().Add(nodeExpiry)

				s.watcher.OnLeave(entry.ID)
			} else if e.Key == compactKey {
				// If we get a compact key we know we can discard all versions
				// prior to the value.
				compactVersion, err := strconv.ParseUint(e.Value, 10, 64)
				if err != nil {
					return
				}
				for _, e := range state.Entries {
					if e.Version <= compactVersion {
						s.metricsDeleteEntry(state.ID, e)
						delete(state.Entries, e.Key)

						if !e.Deleted {
							// If we didn't already know the entry was deleted,
							// notify the watcher.
							s.watcher.OnDeleteKey(entry.ID, e.Key)
						}
					}
				}
			}
		} else {
			if e.Deleted {
				s.watcher.OnDeleteKey(entry.ID, e.Key)
			} else {
				s.watcher.OnUpsertKey(entry.ID, e.Key, e.Value)
			}
		}
	}
}

// RemoveExpiredAt removes all expired node state.
func (s *clusterState) RemoveExpired() {
	s.RemoveExpiredAt(time.Now())
}

func (s *clusterState) RemoveExpiredAt(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var nodeIDs []string
	for _, state := range s.nodes {
		if !state.Expiry.IsZero() && t.After(state.Expiry) {
			nodeIDs = append(nodeIDs, state.ID)
		}
	}

	for _, id := range nodeIDs {
		delete(s.nodes, id)

		s.metrics.Entries.DeletePartialMatch(prometheus.Labels{
			"node_id": id,
		})

		s.watcher.OnExpired(id)
		s.failureDetector.Remove(id)
	}
}

func (s *clusterState) UpdateLiveness(suspicionThreshold float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range s.nodes {
		if node.ID == s.localID || node.Left {
			continue
		}

		suspicionLevel := s.failureDetector.SuspicionLevel(node.ID)
		if suspicionLevel > suspicionThreshold {
			if !node.Unreachable {
				node.Unreachable = true
				node.Expiry = time.Now().Add(nodeExpiry)
				s.watcher.OnUnreachable(node.ID)
			}
		} else {
			if node.Unreachable {
				node.Unreachable = false
				node.Expiry = time.Time{}
				s.watcher.OnReachable(node.ID)
			}
		}
	}
}

func (s *clusterState) metricsAddEntry(nodeID string, newEntry Entry) {
	s.metrics.Entries.With(prometheus.Labels{
		"node_id":  nodeID,
		"internal": strconv.FormatBool(newEntry.Internal),
		"deleted":  strconv.FormatBool(newEntry.Deleted),
	}).Inc()
}

func (s *clusterState) metricsDeleteEntry(nodeID string, existingEntry Entry) {
	s.metrics.Entries.With(prometheus.Labels{
		"node_id":  nodeID,
		"internal": strconv.FormatBool(existingEntry.Internal),
		"deleted":  strconv.FormatBool(existingEntry.Deleted),
	}).Dec()
}

func (s *clusterState) metricsUpsertEntry(nodeID string, newEntry Entry, existingEntry Entry) {
	if existingEntry.Key != "" {
		s.metricsDeleteEntry(nodeID, existingEntry)
	}
	s.metricsAddEntry(nodeID, newEntry)
}
