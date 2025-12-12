package discovery

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// helper: create a PeerStore pointing to a temp file.
func newTestPeerStore(t *testing.T) *PeerStore {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")

	return NewPeerStore(path)
}

// This harness stresses PeerStore under heavy concurrent read/write,
// similar to what Manager + LAN discovery would do.
func TestPeerStoreRaceHarness(t *testing.T) {
	ps := newTestPeerStore(t)

	const (
		addrs        = 20
		writerGorout = 10
		readerGorout = 5
	)

	var wg sync.WaitGroup

	// Writers: repeatedly mark successes and failures on a set of addresses.
	for i := 0; i < writerGorout; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				addr := makeAddr(idx, j, addrs)
				if j%2 == 0 {
					ps.NoteSuccess(addr)
				} else {
					ps.NoteFailure(addr)
				}
			}
		}(i)
	}

	// Readers: repeatedly call Candidates while writers are active.
	for range writerGorout {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				_ = ps.Candidates(3)
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// small extra read at the end
	_ = ps.Candidates(3)

	// ensure file exists (save() was called correctly)
	if _, err := os.Stat(ps.path); err != nil {
		t.Fatalf("peerstore file not written: %v", err)
	}
}

// construct a pseudo address string just for the store.
func makeAddr(i, j, mod int) string {
	port := 10000 + (i+j)%mod
	return "127.0.0.1:" + strconv.Itoa(port)
}
