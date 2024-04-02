package rpc

import (
	"encoding/binary"
	"fmt"
)

const (
	headerSize = 12

	bit1 flags = 1 << 15
	bit2 flags = 1 << 14
)

// flags is a bitset of message flags.
//
// From high order bit down, flags contains:
// - Request/Response: 0 if the message is a request, 1 if the message is a
// response
// - Not supported: 1 if the no handler for the requested RPC type is found
type flags uint16

// Response returns true if the message is a response, false if the message is
// a request.
func (f *flags) Response() bool {
	return *f&bit1 != 0
}

func (f *flags) SetResponse() {
	if f.Response() {
		return
	}
	*f |= bit1
}

// ErrNotSupported returns true if the requested RPC type was not supported by
// the receiver.
func (f *flags) ErrNotSupported() bool {
	return *f&bit2 != 0
}

func (f *flags) SetErrNotSupported() {
	if f.ErrNotSupported() {
		return
	}
	*f |= bit2
}

type header struct {
	// RPCType contains the application RPC type, such as 'heartbeat'.
	RPCType Type

	// ID uniquely identifies the request/response pair.
	ID uint64

	// Flags contains a bitset of flags.
	Flags flags
}

func (h *header) Encode() []byte {
	b := make([]byte, headerSize)
	binary.BigEndian.PutUint16(b, uint16(h.RPCType))
	binary.BigEndian.PutUint64(b[2:], h.ID)
	binary.BigEndian.PutUint16(b[10:], uint16(h.Flags))
	return b
}

func (h *header) Decode(b []byte) error {
	if len(b) < headerSize {
		return fmt.Errorf("message too small: %d", len(b))
	}
	h.RPCType = Type(binary.BigEndian.Uint16(b))
	h.ID = binary.BigEndian.Uint64(b[2:])
	h.Flags = flags(binary.BigEndian.Uint16(b[10:]))
	return nil
}
