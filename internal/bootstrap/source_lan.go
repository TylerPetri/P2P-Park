package bootstrap

import (
	"context"

	"p2p-park/internal/discovery"
	"p2p-park/internal/netx"
)

type LANSource struct {
	Cfg        discovery.LANConfig
	ListenAddr string
	NameStr    string
}

func (s LANSource) Name() string { return "lan" }

func (s LANSource) Discover(ctx context.Context) ([]netx.Addr, error) {
	addrs, err := discovery.DiscoverLANPeers(s.Cfg, s.ListenAddr, s.NameStr)
	if err != nil {
		return nil, err
	}
	out := make([]netx.Addr, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, netx.Addr(a))
	}
	return out, nil
}
