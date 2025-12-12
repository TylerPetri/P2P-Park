package points

import (
	"crypto/ed25519"
	"sync"
	"testing"
	"time"
)

// This test is a concurrency/race harness for Engine. It does not check
// exact point totals; it is meant for go test -race.
func TestEngineRaceHarness(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	eng := NewEngine("self", priv, pub)

	const (
		writers       = 4
		readers       = 4
		remoteSenders = 2
		loops         = 1000
	)

	var wg sync.WaitGroup

	// Writers: bump self points.
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				// We don't care about the result here, just the concurrency.
				_, _ = eng.AddSelf(1)
			}
		}()
	}

	// Remote senders: take self snapshot and feed it back as "remote".
	// This exercises ApplyRemote and the `others` map concurrently.
	for range remoteSenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				snap, _ := eng.SnapshotSelf()
				_ = eng.ApplyRemote(snap)
			}
		}()
	}

	// Readers: SnapshotSelf + All in a loop.
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				_, _ = eng.SnapshotSelf()
				_ = eng.All()
			}
		}()
	}

	wg.Wait()
}
