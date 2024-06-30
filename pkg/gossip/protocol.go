package gossip

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/ugorji/go/codec"
)

type messageType uint8

const (
	messageTypeDigest messageType = iota + 1
	messageTypeDelta
	messageTypeJoin
	messageTypeLeave
)

func (t messageType) String() string {
	switch t {
	case messageTypeDigest:
		return "digest"
	case messageTypeDelta:
		return "delta"
	case messageTypeJoin:
		return "join"
	case messageTypeLeave:
		return "leave"
	default:
		return "unknown"
	}
}

const (
	supportedVersion uint8 = 0
)

// trackedWriter is a wrapper for the underlying writer that counts the number
// of bytes written.
type trackedWriter struct {
	w io.Writer
	n int
}

func newTrackedWriter(w io.Writer) *trackedWriter {
	return &trackedWriter{
		w: w,
		n: 0,
	}
}

func (w *trackedWriter) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	w.n += n
	return n, err
}

func (w *trackedWriter) NumBytesWritten() int {
	return w.n
}

var _ io.Writer = &trackedWriter{}

// trackedReader is a wrapper for the underlying reader that counts the number
// of bytes read.
type trackedReader struct {
	r io.Reader
	n int
}

func newTrackedReader(r io.Reader) *trackedReader {
	return &trackedReader{
		r: r,
	}
}

func (r *trackedReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	r.n += n
	return n, err
}

func (r *trackedReader) NumBytesRead() int {
	return r.n
}

var _ io.Reader = &trackedReader{}

type encoder struct {
	encoder *codec.Encoder
}

func newEncoder(writer io.Writer) *encoder {
	var handle codec.MsgpackHandle
	return &encoder{
		encoder: codec.NewEncoder(writer, &handle),
	}
}

func (e *encoder) Encode(v interface{}) error {
	return e.encoder.Encode(v)
}

func encodeDigest(header digestHeader, digest digest, maxPacketSize int) ([]byte, error) {
	// Add fixed header.
	var buf bytes.Buffer
	_ = buf.WriteByte(uint8(messageTypeDigest))
	_ = buf.WriteByte(supportedVersion)

	encoder := newEncoder(&buf)

	if err := encoder.Encode(&header); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	if buf.Len() > maxPacketSize {
		return nil, fmt.Errorf(
			"max packet size too small for header: %d < %d",
			maxPacketSize, buf.Len(),
		)
	}

	// Keep appending digest entries until we exceed the max packet size.
	// bufLen contains the number of bytes to send (which may be less than
	// buf.Len() if we exceed the packet limit).
	bufLen := buf.Len()
	for _, entry := range digest {
		if err := encoder.Encode(&entry); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}

		if buf.Len() > maxPacketSize {
			break
		}
		bufLen = buf.Len()
	}

	return buf.Bytes()[:bufLen], nil
}

func encodeDelta(header deltaHeader, delta delta, maxPacketSize int) ([]byte, error) {
	// Add fixed header.
	var buf bytes.Buffer
	_ = buf.WriteByte(uint8(messageTypeDelta))
	_ = buf.WriteByte(supportedVersion)

	encoder := newEncoder(&buf)

	if err := encoder.Encode(&header); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	if buf.Len() > maxPacketSize {
		return nil, fmt.Errorf(
			"max packet size too small for header: %d < %d",
			maxPacketSize, buf.Len(),
		)
	}

	// Keep appending delta entries until we exceed the max packet size.
	// bufLen contains the number of bytes to send (which may be less than
	// buf.Len() if we exceed the packet limit).
	bufLen := buf.Len()
	entriesSent := 0
	for _, deltaEntry := range delta {
		if err := encoder.Encode(&deltaHeader{
			NodeID:  deltaEntry.ID,
			Addr:    deltaEntry.Addr,
			Entries: len(deltaEntry.Entries),
		}); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}

		if buf.Len() > maxPacketSize {
			break
		}
		bufLen = buf.Len()

		for _, entry := range deltaEntry.Entries {
			if err := encoder.Encode(entry); err != nil {
				return nil, fmt.Errorf("encode: %w", err)
			}

			if buf.Len() > maxPacketSize {
				break
			}
			bufLen = buf.Len()
			entriesSent++
		}
	}

	return buf.Bytes()[:bufLen], nil
}

type decoder struct {
	decoder *codec.Decoder
}

func newDecoder(reader io.Reader) *decoder {
	var handle codec.MsgpackHandle
	return &decoder{
		decoder: codec.NewDecoder(reader, &handle),
	}
}

func (d *decoder) Decode(v interface{}) error {
	return d.decoder.Decode(v)
}

func decodeDigest(b []byte) (digestHeader, digest, error) {
	r := bytes.NewBuffer(b)

	firstByte, err := r.ReadByte()
	if err != nil {
		return digestHeader{}, nil, fmt.Errorf("read: %w", err)
	}
	messageType := messageType(firstByte)
	if messageType != messageTypeDigest {
		return digestHeader{}, nil, fmt.Errorf("incorrect message type: %s", messageType)
	}
	version, err := r.ReadByte()
	if err != nil {
		return digestHeader{}, nil, fmt.Errorf("read: %w", err)
	}
	if version != supportedVersion {
		return digestHeader{}, nil, fmt.Errorf("unsupported version: %d", version)
	}

	decoder := newDecoder(r)
	var header digestHeader
	if err := decoder.Decode(&header); err != nil {
		return digestHeader{}, nil, fmt.Errorf("decode: %w", err)
	}

	var digest digest
	for {
		// Read digest entries until EOF.
		var entry digestEntry
		if err := decoder.Decode(&entry); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return digestHeader{}, nil, fmt.Errorf("decode: %w", err)
		}
		digest = append(digest, entry)
	}

	return header, digest, nil
}

func decodeDelta(b []byte) (deltaHeader, delta, error) {
	r := bytes.NewBuffer(b)

	firstByte, err := r.ReadByte()
	if err != nil {
		return deltaHeader{}, nil, fmt.Errorf("read: %w", err)
	}
	messageType := messageType(firstByte)
	if messageType != messageTypeDelta {
		return deltaHeader{}, nil, fmt.Errorf("incorrect message type: %s", messageType)
	}
	version, err := r.ReadByte()
	if err != nil {
		return deltaHeader{}, nil, fmt.Errorf("read: %w", err)
	}
	if version != supportedVersion {
		return deltaHeader{}, nil, fmt.Errorf("unsupported version: %d", version)
	}

	decoder := newDecoder(r)
	var header deltaHeader
	if err := decoder.Decode(&header); err != nil {
		return deltaHeader{}, nil, fmt.Errorf("decode: %w", err)
	}

	var delta delta
	for {
		// Read delta entries until EOF.
		var entryHeader deltaHeader
		if err := decoder.Decode(&entryHeader); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return deltaHeader{}, nil, fmt.Errorf("decode: %w", err)
		}

		deltaEntry := deltaEntry{
			ID:   entryHeader.NodeID,
			Addr: entryHeader.Addr,
		}

		// Read entries until we hit the number of entries from the header
		// or EOF.
		for i := 0; i != entryHeader.Entries; i++ {
			var entry Entry
			if err := decoder.Decode(&entry); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return deltaHeader{}, nil, fmt.Errorf("decode: %w", err)
			}

			deltaEntry.Entries = append(deltaEntry.Entries, entry)
		}

		delta = append(delta, deltaEntry)
	}

	return header, delta, nil
}

type digestHeader struct {
	NodeID  string `codec:"node_id"`
	Addr    string `codec:"addr"`
	Request bool   `codec:"request"`
}

type deltaHeader struct {
	NodeID  string `codec:"node_id"`
	Addr    string `codec:"addr"`
	Entries int    `codec:"entries"`
}

type joinHeader struct {
	NodeID string `codec:"node_id"`
	Addr   string `codec:"addr"`
}

type leaveHeader struct {
	NodeID string `codec:"node_id"`
	Addr   string `codec:"addr"`
}
