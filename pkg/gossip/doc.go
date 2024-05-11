// Package gossip manages cluster membership, anti-entropy and failure
// detection for the local node.
//
// At the gossip layer, a nodes state is represented as string key-value pairs
// which will be gossiped to the other nodes in the cluster. Therefore each
// node will have an eventually consistent view of the other nodes state.
package gossip
