package bootstrap

import (
	"context"
	"p2p-park/internal/netx"
)

type PeerSource interface {
	// Discover returns candidate peers to connect to.
	Discover(ctx context.Context) ([]netx.Addr, error)
	Name() string
}
