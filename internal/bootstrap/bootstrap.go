package bootstrap

import (
	"context"
	"math/rand"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
)

type Config struct {
	MaxConnectPerRound int
	PerAddrTimeout     time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxConnectPerRound: 12,
		PerAddrTimeout:     2 * time.Second,
	}
}

// RunOnce gathers candidates from sources and attempts connections.
func RunOnce(ctx context.Context, n *p2p.Node, cfg Config, sources ...PeerSource) {
	cands := make([]netx.Addr, 0, 64)

	for _, s := range sources {
		addrs, err := s.Discover(ctx)
		if err != nil {
			n.Logf("[bootstrap] %s discover error: %v", s.Name(), err)
			continue
		}
		cands = append(cands, addrs...)
	}

	// Shuffle to avoid everyone hitting the same bootstrap in the same order.
	rand.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })

	// Dedup + attempt connects
	seen := make(map[string]struct{}, len(cands))
	connected := 0

	for _, a := range cands {
		if connected >= cfg.MaxConnectPerRound {
			break
		}
		key := string(a)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		// Optional: per-address timeout wrapper
		_ = n.ConnectTo(a)
		connected++
	}
}
