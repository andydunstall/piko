package proxy

import "github.com/andydunstall/pico/pkg/rpc"

type endpoint struct {
	streams   []*rpc.Stream
	nextIndex int
}

func (e *endpoint) AddUpstream(s *rpc.Stream) {
	e.streams = append(e.streams, s)
}

func (e *endpoint) RemoveUpstream(s *rpc.Stream) bool {
	for i := 0; i != len(e.streams); i++ {
		if e.streams[i] == s {
			e.streams = append(e.streams[:i], e.streams[i+1:]...)
			if len(e.streams) == 0 {
				return true
			}
			e.nextIndex %= len(e.streams)
			return false
		}
	}
	return len(e.streams) == 0
}

func (e *endpoint) Next() *rpc.Stream {
	if len(e.streams) == 0 {
		return nil
	}

	s := e.streams[e.nextIndex]
	e.nextIndex++
	e.nextIndex %= len(e.streams)
	return s
}
