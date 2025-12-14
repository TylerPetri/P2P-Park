package p2p

import (
	"sync"
	"testing"
	"time"

	"p2p-park/internal/proto"
)

// TestGossipRaceHarness is a small scenario designed to exercise concurrency
// under `go test -race`, not to assert business logic.
func TestGossipRaceHarness(t *testing.T) {
	n1 := newTestNode(t, "n1")
	n2 := newTestNode(t, "n2")

	if err := n2.ConnectTo(n1.ListenAddr()); err != nil {
		t.Fatalf("ConnectTo error: %v", err)
	}

	waitPeers(t, n1, 1, 5*time.Second)
	waitPeers(t, n2, 1, 5*time.Second)

	done := make(chan struct{})
	defer close(done)

	drainIncomingForever(t, n1, done)
	drainIncomingForever(t, n2, done)

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
		ID:      NewMsgID(),
		Channel: "test",
		Body:    proto.MustMarshal(msg1),
	}
	g2 := proto.Gossip{
		ID:      NewMsgID(),
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
