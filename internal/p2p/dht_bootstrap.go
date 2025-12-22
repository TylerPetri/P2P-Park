package p2p

import (
	"encoding/hex"
	"math/rand"
	"time"

	"p2p-park/internal/dht"
	"p2p-park/internal/netx"
)

type DHTBootstrapConfig struct {
	MinPeers       int
	Tick           time.Duration
	PerTickLookups int
	Lookup         dht.LookupConfig
}

func DefaultDHTBootstrapConfig() DHTBootstrapConfig {
	return DHTBootstrapConfig{
		MinPeers:       6,
		Tick:           3 * time.Second,
		PerTickLookups: 2,
		Lookup:         dht.DefaultLookupConfig(),
	}
}

// startDHTBootstrapLoop runs after Start() if n has a DHT handler.
// It expands the peer set using iterative FIND_NODE and then dials learned addrs.
func (n *Node) startDHTBootstrapLoop(cfg DHTBootstrapConfig) {
	if n.dht == nil {
		return
	}

	if cfg.MinPeers <= 0 {
		cfg.MinPeers = 6
	}
	if cfg.Tick <= 0 {
		cfg.Tick = 3 * time.Second
	}
	if cfg.PerTickLookups <= 0 {
		cfg.PerTickLookups = 2
	}

	go func() {
		t := time.NewTicker(cfg.Tick)
		defer t.Stop()

		for {
			select {
			case <-n.ctx.Done():
				return
			case <-t.C:
				// Need at least one connected peer to do anything.
				if n.PeerCount() == 0 {
					continue
				}
				if n.PeerCount() >= cfg.MinPeers {
					continue
				}

				// Run a few lookups per tick.
				for i := 0; i < cfg.PerTickLookups; i++ {
					target := randomNodeIDHex()
					nodes, err := n.dht.IterativeFindNode(n, target, cfg.Lookup)
					if err != nil {
						continue
					}

					// Dial what we learned.
					for _, ni := range nodes {
						if ni.NodeID == "" || ni.Addr == "" {
							continue
						}
						if ni.NodeID == n.ID() {
							continue
						}
						if n.hasPeer(ni.NodeID) {
							continue
						}
						_ = n.ConnectTo(netx.Addr(ni.Addr))
					}
				}
			}
		}
	}()
}

func (n *Node) coldStartDHTBootstrap() {
	if n.dht == nil || len(n.cfg.Bootstraps) != 0 {
		return
	}

	for _, addr := range n.dht.BootstrapAddrs(8) {
		if addr == "" {
			continue
		}
		_ = n.ConnectTo(netx.Addr(addr))
	}
}

func randomNodeIDHex() string {
	// 32 bytes -> 256-bit ID; matches your NodeID/XOR style.
	var b [32]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
