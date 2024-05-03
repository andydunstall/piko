package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalEndpoint(t *testing.T) {
	endpoint := &localEndpoint{}

	assert.Nil(t, endpoint.Next())

	conn1 := &fakeConn{addr: "1"}
	endpoint.AddConn(conn1)
	assert.Equal(t, "1", endpoint.Next().Addr())

	conn2 := &fakeConn{addr: "2"}
	conn3 := &fakeConn{addr: "3"}
	conn4 := &fakeConn{addr: "4"}
	endpoint.AddConn(conn2)
	endpoint.AddConn(conn3)
	endpoint.AddConn(conn4)

	assert.Equal(t, "1", endpoint.Next().Addr())
	assert.Equal(t, "2", endpoint.Next().Addr())
	assert.Equal(t, "3", endpoint.Next().Addr())
	assert.Equal(t, "4", endpoint.Next().Addr())
	assert.Equal(t, "1", endpoint.Next().Addr())
	assert.Equal(t, "2", endpoint.Next().Addr())
	assert.Equal(t, "3", endpoint.Next().Addr())

	assert.False(t, endpoint.RemoveConn(conn2))
	assert.False(t, endpoint.RemoveConn(conn3))
	assert.Equal(t, "1", endpoint.Next().Addr())
	assert.Equal(t, "4", endpoint.Next().Addr())
	assert.Equal(t, "1", endpoint.Next().Addr())

	assert.False(t, endpoint.RemoveConn(conn1))
	assert.True(t, endpoint.RemoveConn(conn4))

	assert.Nil(t, endpoint.Next())
}
