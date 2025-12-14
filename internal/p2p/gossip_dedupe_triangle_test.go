package p2p

import (
	"encoding/json"
	"testing"
	"time"

	"p2p-park/internal/proto"
)

func TestGossipDedupe_NoLoopTriangle(t *testing.T) {
	a := newTestNode(t, "a")
	b := newTestNode(t, "b")
	c := newTestNode(t, "c")

	connectTriangle(t, a, b, c)

	waitPeers(t, a, 2, 3*time.Second)
	waitPeers(t, b, 2, 3*time.Second)
	waitPeers(t, c, 2, 3*time.Second)

	// Send ONE gossip with a fixed ID from A.
	id := "fixed-triangle-id"
	g := proto.Gossip{
		ID:      id,
		Channel: "enc:test",   // content doesn't matter; loop happens in gossip layer
		Body:    []byte(`{}`), // payload irrelevant for dedupe test
	}
	a.Broadcast(g)

	countB := 0
	countC := 0

	// First, wait until both B and C have seen it at least once.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		// drain
		for {
			select {
			case env := <-b.Incoming():
				if env.Type == proto.MsgGossip {
					var gg proto.Gossip
					_ = json.Unmarshal(env.Payload, &gg)
					if gg.ID == id {
						countB++
					}
				}
			default:
				goto drainedB
			}
		}
	drainedB:

		for {
			select {
			case env := <-c.Incoming():
				if env.Type == proto.MsgGossip {
					var gg proto.Gossip
					_ = json.Unmarshal(env.Payload, &gg)
					if gg.ID == id {
						countC++
					}
				}
			default:
				goto drainedC
			}
		}
	drainedC:

		if countB >= 1 && countC >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if countB < 1 || countC < 1 {
		t.Fatalf("expected gossip to arrive: B=%d C=%d", countB, countC)
	}

	time.Sleep(250 * time.Millisecond)

	// Drain again after quiet window.
	for {
		select {
		case env := <-b.Incoming():
			if env.Type == proto.MsgGossip {
				var gg proto.Gossip
				_ = json.Unmarshal(env.Payload, &gg)
				if gg.ID == id {
					countB++
				}
			}
		default:
			goto drainedB2
		}
	}
drainedB2:

	for {
		select {
		case env := <-c.Incoming():
			if env.Type == proto.MsgGossip {
				var gg proto.Gossip
				_ = json.Unmarshal(env.Payload, &gg)
				if gg.ID == id {
					countC++
				}
			}
		default:
			goto drainedC2
		}
	}
drainedC2:

	if countB != 1 || countC != 1 {
		t.Fatalf("dedupe failed (loop/dup detected): B=%d C=%d (expected 1 each)", countB, countC)
	}
}
