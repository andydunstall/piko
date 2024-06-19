package upstream

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeUpstream struct {
	endpointID string
}

func (u *fakeUpstream) EndpointID() string {
	return u.endpointID
}

func (u *fakeUpstream) Dial() (net.Conn, error) {
	return nil, nil
}

func (u *fakeUpstream) Forward() bool {
	return false
}

func TestLocalLoadBalancer(t *testing.T) {
	lb := &loadBalancer{}

	assert.Nil(t, lb.Next())

	u1 := &fakeUpstream{endpointID: "1"}
	lb.Add(u1)
	assert.Equal(t, "1", lb.Next().EndpointID())

	u2 := &fakeUpstream{endpointID: "2"}
	u3 := &fakeUpstream{endpointID: "3"}
	u4 := &fakeUpstream{endpointID: "4"}
	lb.Add(u2)
	lb.Add(u3)
	lb.Add(u4)

	assert.Equal(t, "1", lb.Next().EndpointID())
	assert.Equal(t, "2", lb.Next().EndpointID())
	assert.Equal(t, "3", lb.Next().EndpointID())
	assert.Equal(t, "4", lb.Next().EndpointID())
	assert.Equal(t, "1", lb.Next().EndpointID())
	assert.Equal(t, "2", lb.Next().EndpointID())
	assert.Equal(t, "3", lb.Next().EndpointID())

	assert.False(t, lb.Remove(u2))
	assert.False(t, lb.Remove(u3))
	assert.Equal(t, "1", lb.Next().EndpointID())
	assert.Equal(t, "4", lb.Next().EndpointID())
	assert.Equal(t, "1", lb.Next().EndpointID())

	assert.False(t, lb.Remove(u1))
	assert.True(t, lb.Remove(u4))

	assert.Nil(t, lb.Next())
}
