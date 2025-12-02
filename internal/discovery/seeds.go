package discovery

import "p2p-park/internal/p2p"

var DefaultSeeds = []string{
	// "seed1.p2p-park.net:4001",
	// "seed2.p2p-park.net:4001",
}

type SeedStrategy struct{}

func (s *SeedStrategy) Discover(_ *p2p.Node) []string {
	return append([]string{}, DefaultSeeds...)
}
