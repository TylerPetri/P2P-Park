package discovery

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
)

// fakeLAN simulates LAN broadcast by sharing addresses in memory.
type fakeLAN struct {
	mu    sync.RWMutex
	addrs []netx.Addr
}

func (f *fakeLAN) announce(addr netx.Addr) {
	f.mu.Lock()
	f.addrs = append(f.addrs, addr)
	f.mu.Unlock()
}

func (f *fakeLAN) snapshot() []netx.Addr {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]netx.Addr, len(f.addrs))
	copy(out, f.addrs)
	return out
}

// This harness simulates LAN discovery at scale without UDP.
// It focuses on concurrency correctness, not packet correctness.
func TestLANDiscoveryRaceHarness(t *testing.T) {
	const nodes = 5

	lan := &fakeLAN{}
	var all []*p2p.Node

	// Create nodes
	for i := 0; i < nodes; i++ {

		n, err := p2p.NewNode(p2p.NodeConfig{
			Name:       fmt.Sprintf("node-%d", i),
			Network:    netx.NewTCPNetwork(),
			BindAddr:   "127.0.0.1:0",
			Bootstraps: nil,
			Protocol:   "test/0",
			Debug:      true,
		})
		if err != nil {
			t.Fatalf("NewNode: %v", err)
		}
		if err := n.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		all = append(all, n)
		lan.announce(n.ListenAddr())
	}

	defer func() {
		for _, n := range all {
			n.Stop()
		}
	}()

	const loops = 100
	var wg sync.WaitGroup
	wg.Add(nodes)

	// Each node concurrently "discovers" everyone else
	for _, n := range all {
		go func(n *p2p.Node) {
			defer wg.Done()
			for range loops {
				for _, addr := range lan.snapshot() {
					// Skip self-connect if your code does that internally
					_ = n.ConnectTo(addr)
				}
			}
		}(n)
	}

	wg.Wait()

	// Let connections settle
	time.Sleep(200 * time.Millisecond)

	// Sanity: everyone should see at least one peer
	for _, n := range all {
		if n.PeerCount() == 0 {
			t.Fatalf("node %s (%s) discovered no peers", n.ID(), n.ListenAddr())
		}
	}
}
