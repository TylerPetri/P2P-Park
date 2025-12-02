package discovery

import "p2p-park/internal/p2p"

type Strategy interface {
	Discover(n *p2p.Node) []string // returns list of addresses to try
}
