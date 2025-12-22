package p2p

import (
	"p2p-park/internal/dht"
	"testing"
	"time"
)

func TestDHT_QueryFindNode_Triangle(t *testing.T) {
	a := newTestNode(t, "a")
	b := newTestNode(t, "b")
	c := newTestNode(t, "c")

	connectTriangle(t, a, b, c)

	if a.dht == nil || b.dht == nil || c.dht == nil {
		t.Fatalf("expected dht enabled on all nodes")
	}

	cNodeID, err := dht.NodeIDFromPeerID(c.ID())
	if err != nil {
		t.Fatalf("failed to derive node id from peer id: %v", err)
	}

	resp, err := a.dhtAccessor().QueryFindNode(a, b.ID(), cNodeID.Hex(), 1500*time.Millisecond)
	if err != nil {
		t.Fatalf("QueryFindNode error: %v", err)
	}

	found := false
	for _, nd := range resp.Nodes {
		if nd.PeerID == c.ID() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected response to include target peer %s; nodes=%+v", c.ID(), resp.Nodes)
	}
}
