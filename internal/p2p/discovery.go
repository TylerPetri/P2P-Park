package p2p

import (
	"p2p-park/internal/netx"
	"time"
)

func (n *Node) discoveryLoop() {
	// On startup, try bootstraps.
	for _, addr := range n.cfg.Bootstraps {
		if addr == "" {
			continue
		}
		n.Logf("bootstrap: dialing %s", addr)
		n.ConnectTo(addr)
	}

	// Periodically attempt to reconnect / expand the graph.
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.discoveryTick()
		}
	}
}

func (n *Node) discoveryTick() {
	if len(n.cfg.Bootstraps) == 0 {
		return
	}
	for _, addr := range n.cfg.Bootstraps {
		if addr == netx.Addr("") {
			continue
		}
		if n.hasPeer(string(addr)) {
			continue
		}
		n.Logf("discovery: dial bootstrap %s", addr)
		n.ConnectTo(addr)
	}
}
