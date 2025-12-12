package p2p

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"p2p-park/internal/crypto/channel"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
)

// Make a node for tests (noisy logs off).
func newEncTestNode(t *testing.T, name string) *Node {
	t.Helper()

	logger := log.New(io.Discard, "", 0)

	n, err := NewNode(NodeConfig{
		Name:       name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   "127.0.0.1:0",
		Bootstraps: nil,
		Protocol:   "test/0",
		Logger:     logger,
		Debug:      true,
		IsSeed:     false,
	})
	if err != nil {
		t.Fatalf("NewNode(%s) error: %v", name, err)
	}
	if err := n.Start(); err != nil {
		t.Fatalf("Start(%s) error: %v", name, err)
	}
	return n
}

// drain incoming so writers don't stall if channels fill.
// We still "inspect" enc:gossip messages to stress decrypt.
func drainAndDecrypt(
	t *testing.T,
	n *Node,
	encMu *sync.RWMutex,
	encChannels map[string]channel.ChannelKey,
	done <-chan struct{},
	decryptCount *int64,
) {
	t.Helper()

	go func() {
		for {
			select {
			case <-done:
				return
			case env, ok := <-n.Incoming():
				if !ok {
					return
				}
				// Only care about Gossip messages.
				if env.Type != proto.MsgGossip {
					continue
				}

				var g proto.Gossip
				// only a stress test for now, if want descrypt we can add that later
				_ = g
				_ = decryptCount
			}
		}
	}()
}

// This test doesn't aim to validate cryptographic correctness — it's a race/stress harness.
// It simulates heavy concurrent encrypted channel usage (create/join/encrypt/broadcast)
// while draining incoming so backpressure doesn't hide races.
func TestEncryptedChannelStressRaceHarness(t *testing.T) {
	n1 := newEncTestNode(t, "n1")
	defer n1.Stop()

	n2 := newEncTestNode(t, "n2")
	defer n2.Stop()

	if err := n2.ConnectTo(n1.ListenAddr()); err != nil {
		t.Fatalf("ConnectTo error: %v", err)
	}

	// Wait until both sides see each other.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if n1.PeerCount() >= 1 && n2.PeerCount() >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for peers: n1=%d n2=%d", n1.PeerCount(), n2.PeerCount())
		}
		time.Sleep(10 * time.Millisecond)
	}

	enc1 := make(map[string]channel.ChannelKey)
	enc2 := make(map[string]channel.ChannelKey)
	var mu1 sync.RWMutex
	var mu2 sync.RWMutex

	// Create a bunch of channels and share keys between nodes.
	const chCount = 10
	for i := 0; i < chCount; i++ {
		chName := "chan-" + hex.EncodeToString([]byte{byte(i)})

		k, err := channel.NewRandomKey()
		if err != nil {
			t.Fatalf("NewRandomKey: %v", err)
		}

		// n1 creates
		mu1.Lock()
		enc1[chName] = k
		mu1.Unlock()

		// n2 joins (same key)
		mu2.Lock()
		enc2[chName] = k
		mu2.Unlock()
	}

	done := make(chan struct{})
	defer close(done)

	// Drain incoming on both nodes.
	drainAndDecrypt(t, n1, &mu1, enc1, done, nil)
	drainAndDecrypt(t, n2, &mu2, enc2, done, nil)

	// Stress: concurrently mutate encChannels (simulate /mkchan /joinchan)
	// while also encrypting + broadcasting.
	const (
		senders = 8
		writers = 2
		loops   = 200
	)

	var wg sync.WaitGroup
	wg.Add(senders + writers)

	// Writers mutate the map (join/rotate keys) while traffic is ongoing.
	for w := 0; w < writers; w++ {
		go func(idx int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				chName := "chan-" + hex.EncodeToString([]byte{byte(i % chCount)})

				k, err := channel.NewRandomKey()
				if err != nil {
					continue
				}

				// Rotate key in both maps (simulate both nodes re-keying / re-joining)
				mu1.Lock()
				enc1[chName] = k
				mu1.Unlock()

				mu2.Lock()
				enc2[chName] = k
				mu2.Unlock()
			}
		}(w)
	}

	// Senders encrypt and broadcast on random channels.
	for s := 0; s < senders; s++ {
		go func(idx int) {
			defer wg.Done()

			for i := 0; i < loops; i++ {
				chName := "chan-" + hex.EncodeToString([]byte{byte((idx + i) % chCount)})

				// n1 send
				mu1.RLock()
				k1, ok1 := enc1[chName]
				mu1.RUnlock()

				if ok1 {
					nonce, ct, err := channel.Encrypt(k1, []byte("hi from n1"))
					em := proto.EncryptedMessage{
						Nonce:      nonce,
						Ciphertext: ct,
					}
					body, _ := json.Marshal(em)
					if err == nil {
						g := proto.Gossip{
							Channel: "enc:" + chName,
							Body:    body,
						}
						n1.Broadcast(g)
					}
				}

				// n2 send
				mu2.RLock()
				k2, ok2 := enc2[chName]
				mu2.RUnlock()

				if ok2 {
					nonce, ct, err := channel.Encrypt(k2, []byte("hi from n1"))
					em := proto.EncryptedMessage{
						Nonce:      nonce,
						Ciphertext: ct,
					}
					body, _ := json.Marshal(em)
					if err == nil {
						g := proto.Gossip{
							Channel: "enc:" + chName,
							Body:    body,
						}
						n1.Broadcast(g)
					}
				}
			}
		}(s)
	}

	wg.Wait()

	// Let in-flight sends drain
	time.Sleep(100 * time.Millisecond)
}
