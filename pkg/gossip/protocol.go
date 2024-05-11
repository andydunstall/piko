package gossip

import (
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

type digestHeader struct {
	NodeID  string `codec:"node_id"`
	Addr    string `codec:"addr"`
	Request bool   `codec:"request"`
}

type deltaHeader struct {
	NodeID string `codec:"node_id"`
	Addr   string `codec:"addr"`
}

type joinHeader struct {
	NodeID string `codec:"node_id"`
	Addr   string `codec:"addr"`
}

type leaveHeader struct {
	NodeID string `codec:"node_id"`
	Addr   string `codec:"addr"`
}
