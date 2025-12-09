package p2p

import (
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
)

// newTestNode spins up a node bound to an ephemeral localhost port.
func newTestNode(t *testing.T, name string) *Node {
	t.Helper()

	logger := log.New(io.Discard, "", log.LstdFlags)

	n, err := NewNode(NodeConfig{
		Name:       name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   "127.0.0.1:0",
		Bootstraps: nil,
		Protocol:   "test/0",
		Logger:     logger,
		Debug:      true,
		// IsSeed: false,
	})
	if err != nil {
		t.Fatalf("NewNode(%s) error: %v", name, err)
	}

	if err := n.Start(); err != nil {
		t.Fatalf("Start(%s) error: %v", name, err)
	}

	return n
}

// waitForPeers waits until a node sees at least `want` peers or times out.
func waitForPeers(t *testing.T, n *Node, want int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		peers := n.SnapshotPeers()
		if len(peers) >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d peers, got %d", want, len(peers))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// drainIncoming keeps reading from Incoming() so it doesn’t block writers.
func drainIncoming(t *testing.T, n *Node, done <-chan struct{}) {
	t.Helper()

	go func() {
		for {
			select {
			case <-done:
				return
			case _, ok := <-n.Incoming():
				if !ok {
					return
				}
				// We don't care about contents here; this is a race harness.
			}
		}
	}()
}

// TestGossipRaceHarness is a small scenario designed to exercise concurrency
// under `go test -race`, not to assert business logic.
func TestGossipRaceHarness(t *testing.T) {
	n1 := newTestNode(t, "n1")
	defer n1.Stop()

	n2 := newTestNode(t, "n2")
	defer n2.Stop()

	if err := n2.ConnectTo(n1.ListenAddr()); err != nil {
		t.Fatalf("ConnectTo error: %v", err)
	}

	waitForPeers(t, n1, 1)
	waitForPeers(t, n2, 1)

	done := make(chan struct{})
	defer close(done)

	drainIncoming(t, n1, done)
	drainIncoming(t, n2, done)

	msg1 := proto.ChatMessage{
		Text:      "hello from n1",
		From:      "n1",
		Timestamp: time.Now().Unix(),
	}
	msg2 := proto.ChatMessage{
		Text:      "hello from n2",
		From:      "n2",
		Timestamp: time.Now().Unix(),
	}

	g1 := proto.Gossip{
		Channel: "test",
		Body:    proto.MustMarshal(msg1),
	}
	g2 := proto.Gossip{
		Channel: "test",
		Body:    proto.MustMarshal(msg2),
	}

	const loops = 100

	var wg sync.WaitGroup

	wg.Add(4)

	go func() {
		defer wg.Done()
		for range loops {
			n1.Broadcast(g1)
		}
	}()

	go func() {
		defer wg.Done()
		for range loops {
			n2.Broadcast(g2)
		}
	}()

	// Also hammer SnapshotPeers concurrently to exercise the RWMutex.
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(1 * time.Second)
		for time.Now().Before(deadline) {
			_ = n1.SnapshotPeers()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		deadline := time.Now().Add(1 * time.Second)
		for time.Now().Before(deadline) {
			_ = n2.SnapshotPeers()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Small extra delay to let any in-flight writes finish
	time.Sleep(100 * time.Millisecond)
}
