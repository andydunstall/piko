package gossip

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodec_Digest(t *testing.T) {
	t.Run("full digest", func(t *testing.T) {
		sentHeader := digestHeader{
			NodeID:  "my-node",
			Addr:    "1.2.3.4",
			Request: true,
		}
		sentDigest := digest{
			{"node-1", "1.1.1.1", 4, false},
			{"node-2", "2.2.2.2", 8, false},
			{"node-3", "3.3.3.3", 13, false},
		}

		b, err := encodeDigest(sentHeader, sentDigest, 1000)
		assert.NoError(t, err)

		receivedHeader, receivedDigest, err := decodeDigest(b)
		assert.NoError(t, err)

		assert.Equal(t, sentHeader, receivedHeader)
		assert.Equal(t, sentDigest, receivedDigest)
	})

	// Tests partially encoding a digest due to exceeding the maximum packet
	// length.
	t.Run("partial digest", func(t *testing.T) {
		sentHeader := digestHeader{
			NodeID:  "my-node",
			Addr:    "1.2.3.4",
			Request: true,
		}
		sentDigest := digest{
			{"node-1", "1.1.1.1", 4, false},
			{"node-2", "2.2.2.2", 8, false},
			{"node-3", "3.3.3.3", 13, false},
		}

		b, err := encodeDigest(sentHeader, sentDigest, 125)
		assert.NoError(t, err)
		assert.Equal(t, 119, len(b))

		receivedHeader, receivedDigest, err := decodeDigest(b)
		assert.NoError(t, err)

		assert.Equal(t, sentHeader, receivedHeader)
		assert.Equal(t, digest{
			{"node-1", "1.1.1.1", 4, false},
			{"node-2", "2.2.2.2", 8, false},
		}, receivedDigest)
	})
}

func TestCodec_Delta(t *testing.T) {
	t.Run("full delta", func(t *testing.T) {
		sentHeader := deltaHeader{
			NodeID: "my-node",
			Addr:   "1.2.3.4",
		}
		sentDelta := delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 8, false, false},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "v1", 8, false, false},
					{"k2", "v2", 12, false, false},
					{"k3", "v3", 13, false, false},
				},
			},
		}
		b, err := encodeDelta(sentHeader, sentDelta, 1000)
		assert.NoError(t, err)

		receivedHeader, receivedDelta, err := decodeDelta(b)
		assert.NoError(t, err)

		assert.Equal(t, sentHeader, receivedHeader)
		assert.Equal(t, sentDelta, receivedDelta)
	})

	// Tests partially encoding a delta due to exceeding the maximum packet
	// length.
	t.Run("partial delta", func(t *testing.T) {
		sentHeader := deltaHeader{
			NodeID: "my-node",
			Addr:   "1.2.3.4",
		}
		sentDelta := delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 8, false, false},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "v1", 8, false, false},
					{"k2", "v2", 12, false, false},
					{"k3", "v3", 13, false, false},
				},
			},
		}
		b, err := encodeDelta(sentHeader, sentDelta, 325)
		assert.NoError(t, err)
		assert.Equal(t, 297, len(b))

		receivedHeader, receivedDelta, err := decodeDelta(b)
		assert.NoError(t, err)

		assert.Equal(t, sentHeader, receivedHeader)
		assert.Equal(t, delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 8, false, false},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "v1", 8, false, false},
				},
			},
		}, receivedDelta)
	})
}
