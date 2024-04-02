package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlags_Response(t *testing.T) {
	var f flags
	assert.False(t, f.Response())
	f.SetResponse()
	assert.True(t, f.Response())
}

func TestFlags_ErrNotSupported(t *testing.T) {
	var f flags
	assert.False(t, f.ErrNotSupported())
	f.SetErrNotSupported()
	assert.True(t, f.ErrNotSupported())
}

func TestHeader_Encode(t *testing.T) {
	var flags flags
	flags.SetResponse()
	h := header{
		RPCType: Type(0xff),
		ID:      0x012345678,
		Flags:   flags,
	}
	assert.Equal(t, []byte{0x0, 0xff, 0x0, 0x0, 0x0, 0x0, 0x12, 0x34, 0x56, 0x78, 0x80, 0x0}, h.Encode())
}

func TestHeader_Decode(t *testing.T) {
	var flags flags
	flags.SetResponse()
	h1 := header{
		RPCType: Type(0xff),
		ID:      0x012345678,
		Flags:   flags,
	}
	var h2 header
	assert.NoError(t, h2.Decode(h1.Encode()))
	assert.Equal(t, h1, h2)

	assert.Error(t, h2.Decode([]byte("xxx")))
}
