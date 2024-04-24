// Package gossip is responsible for maintaining this nodes local NetworkMap
// and propagating the state of the local node to the rest of the cluster.
//
// At the gossip layer, a nodes state is represented as key-value pairs which
// are propagated around the cluster using Scuttlebutt, which is an efficient
// gossip based anti-entropy protocol. These key-value pairs are then used to
// build the local NetworkMap.
package gossip
