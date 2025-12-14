package p2p

import (
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"p2p-park/internal/proto"
)

// Wait until seed.natByUserID has at least `want` entries.
func waitForNatRegistry(t *testing.T, seed *Node, want int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		seed.mu.RLock()
		count := len(seed.natByUserID)
		seed.mu.RUnlock()

		if count >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d NAT entries, got %d", want, count)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Helper to find the seed-side *peer for a given node ID.
func findPeerOnSeed(t *testing.T, seed *Node, nodeID string) *peer {
	t.Helper()

	seed.mu.RLock()
	defer seed.mu.RUnlock()

	for _, p := range seed.peers {
		if p != nil && p.id == nodeID {
			return p
		}
	}
	t.Fatalf("seed does not have peer with id %s", nodeID)
	return nil
}

// This harness stresses NAT relay logic on the seed and the per-peer writers
// under go test -race. It’s about concurrency, not semantic correctness.
func TestNATRelayRaceHarness(t *testing.T) {
	seed := newTestNode(t, "seed", WithSeed(true))

	nA := newTestNode(t, "A", WithBootstraps(seed.ListenAddr()))
	nB := newTestNode(t, "B", WithBootstraps(seed.ListenAddr()))

	// Make sure they connect to the seed as well (in addition to Bootstraps).
	if err := nA.ConnectTo(seed.ListenAddr()); err != nil {
		t.Fatalf("A->seed connect: %v", err)
	}
	if err := nB.ConnectTo(seed.ListenAddr()); err != nil {
		t.Fatalf("B->seed connect: %v", err)
	}

	// Wait until the seed has NAT registrations for both A and B.
	waitForNatRegistry(t, seed, 2)

	done := make(chan struct{})
	defer close(done)

	// Drain incoming queues so that NAT relay messages don't block.
	drainIncomingForever(t, seed, done)
	drainIncomingForever(t, nA, done)
	drainIncomingForever(t, nB, done)

	// Build a NatRelay envelope as if A is sending to B.
	aID := nA.ID() // Noise network ID (p.id)
	// NAT "user ID" is hex(SignPub), same as sendNatRegister.
	bUserID := hex.EncodeToString(nB.Identity().SignPub)

	payload := proto.NatRelay{
		ToUserID: bUserID,
		// Valid JSON payload (RawMessage must contain valid JSON).
		Payload: json.RawMessage(`{"msg":"hello from A to B via seed"}`),
	}

	env := proto.Envelope{
		Type:    proto.MsgNatRelay,
		FromID:  aID,
		Payload: proto.MustMarshal(payload),
	}

	fromPeerOnSeed := findPeerOnSeed(t, seed, aID)

	const loops = 100
	var wg sync.WaitGroup
	wg.Add(2)

	// Hammer handleNatRelaySeed (seed forwarding to B).
	go func() {
		defer wg.Done()
		for i := 0; i < loops; i++ {
			seed.handleNatRelaySeed(fromPeerOnSeed, env)
		}
	}()

	// At the same time, hammer SnapshotPeers and PeerCount on all nodes
	// to exercise RWMutexes while NAT relay is active.
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(1 * time.Second)
		for time.Now().Before(deadline) {
			_ = seed.PeerCount()
			_ = nA.PeerCount()
			_ = nB.PeerCount()

			_ = seed.SnapshotPeers()
			_ = nA.SnapshotPeers()
			_ = nB.SnapshotPeers()

			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	// small delay to let any in-flight writes finish
	time.Sleep(50 * time.Millisecond)
}
