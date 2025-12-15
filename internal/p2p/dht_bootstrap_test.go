package p2p

import (
	"testing"
	"time"

	"p2p-park/internal/dht"
	"p2p-park/internal/netx"
)

func TestDHTBootstrap_ExpandsFromOnePeer(t *testing.T) {
	// a knows only b
	a := newTestNode(t, "a")
	b := newTestNode(t, "b")
	c := newTestNode(t, "c")

	// Make b know c directly, so b's routing table learns c via OnPeerSeen(addPeer).
	connect(t, b, c)
	waitPeers(t, b, 1, 2*time.Second)
	waitPeers(t, c, 1, 2*time.Second)

	// Now connect a -> b (a only has b).
	connect(t, a, b)
	waitPeers(t, a, 1, 2*time.Second)

	// Kick a lookup for c's ID and then dial what we learned (like bootstrap loop does).
	nodes, err := a.dhtAccessor().IterativeFindNode(a, c.ID(), dht.DefaultLookupConfig())
	if err != nil {
		t.Fatalf("IterativeFindNode: %v", err)
	}
	for _, ni := range nodes {
		if ni.ID == c.ID() && ni.Addr != "" {
			_ = a.ConnectTo(netx.Addr(ni.Addr))
			break
		}
	}

	// a should eventually have c too.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a.PeerCount() >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected a to expand to 2 peers via DHT bootstrap; have=%d", a.PeerCount())
}
