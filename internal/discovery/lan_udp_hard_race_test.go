package discovery

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
)

func TestLANUDPDiscoveryHardRaceHarness(t *testing.T) {
	if os.Getenv("P2P_LAN_UDP_TEST") == "" {
		t.Skip("set P2P_LAN_UDP_TEST=1 to enable")
	}

	cfg := DefaultLANConfig()

	const nodes = 5
	var all []*p2p.Node
	var stops []chan struct{}

	// Build nodes + start UDP responders
	for i := 0; i < nodes; i++ {
		n, err := p2p.NewNode(p2p.NodeConfig{
			Name:       fmt.Sprintf("udp-node-%d", i),
			Network:    netx.NewTCPNetwork(),
			BindAddr:   "0.0.0.0:0", // prefer v4 for local test consistency
			Bootstraps: nil,
			Protocol:   "test/lan",
			Logger:     log.New(io.Discard, "", 0),
			Debug:      true,
			IsSeed:     false,
		})
		if err != nil {
			t.Fatalf("NewNode: %v", err)
		}
		if err := n.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}

		stop := make(chan struct{})
		if err := StartLANResponder(stop, cfg, string(n.ListenAddr()), n.Name()); err != nil {
			t.Fatalf("StartLANResponder: %v", err)
		}

		all = append(all, n)
		stops = append(stops, stop)
	}

	defer func() {
		for _, ch := range stops {
			close(ch)
		}
		for _, n := range all {
			n.Stop()
		}
	}()

	// Stress discovery+connect in parallel
	const runtime = 3 * time.Second
	deadline := time.Now().Add(runtime)

	var wg sync.WaitGroup
	wg.Add(len(all))

	for _, n := range all {
		go func(n *p2p.Node) {
			defer wg.Done()

			for time.Now().Before(deadline) {
				// Run real UDP discovery
				addrs, err := DiscoverLANPeers(cfg, string(n.ListenAddr()), n.Name())
				if err == nil {
					for _, a := range addrs {
						_ = n.ConnectTo(netx.Addr(a))
					}
				}

				// hammer read paths concurrently
				_ = n.PeerCount()
				_ = n.SnapshotPeers()

				time.Sleep(50 * time.Millisecond)
			}
		}(n)
	}

	wg.Wait()
	time.Sleep(300 * time.Millisecond)

	// HARD assertion: real peers were found via UDP and connected.
	ok := 0
	for _, n := range all {
		if n.PeerCount() > 0 {
			ok++
		}
	}
	if ok < 2 {
		t.Fatalf("expected >=2 nodes to discover/connect via UDP; got %d/%d", ok, len(all))
	}
}
