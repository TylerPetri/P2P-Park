package p2p

import (
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

	// One-hop request: A asks B for nodes close to C.
	resp, err := a.dhtAccessor().QueryFindNode(a, b.ID(), c.ID(), 1500*time.Millisecond)
	if err != nil {
		t.Fatalf("QueryFindNode: %v", err)
	}
	if resp.Kind != "NODES" {
		t.Fatalf("expected NODES, got %q", resp.Kind)
	}

	found := false
	for _, nd := range resp.Nodes {
		if nd.ID == c.ID() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected response to include target %s; nodes=%+v", c.ID(), resp.Nodes)
	}
}
