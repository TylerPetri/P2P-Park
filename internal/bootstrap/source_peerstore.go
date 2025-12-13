package bootstrap

import (
	"context"
	"p2p-park/internal/discovery"
	"p2p-park/internal/netx"
)

type PeerStoreSource struct {
	Store       *discovery.PeerStore
	MaxFailures int
	Limit       int
}

func (s PeerStoreSource) Name() string { return "peerstore" }

func (s PeerStoreSource) Discover(ctx context.Context) ([]netx.Addr, error) {
	c := s.Store.Candidates(s.MaxFailures)
	if s.Limit > 0 && len(c) > s.Limit {
		c = c[:s.Limit]
	}
	out := make([]netx.Addr, 0, len(c))
	for _, a := range c {
		out = append(out, netx.Addr(a))
	}
	return out, nil
}
