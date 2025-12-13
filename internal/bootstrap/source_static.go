package bootstrap

import (
	"context"
	"p2p-park/internal/netx"
)

type StaticSource struct {
	Addrs []netx.Addr
	Label string
}

func (s StaticSource) Name() string {
	if s.Label != "" {
		return s.Label
	}
	return "static"
}

func (s StaticSource) Discover(ctx context.Context) ([]netx.Addr, error) {
	return append([]netx.Addr(nil), s.Addrs...), nil
}
