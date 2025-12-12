package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type peerRecord struct {
	Addr         string    `json:"addr"`
	LastSeen     time.Time `json:"last_seen"`
	LastSuccess  time.Time `json:"last_success"`
	FailureCount int       `json:"failures"`
}

type PeerStore struct {
	path  string
	mu    sync.RWMutex
	peers map[string]*peerRecord
}

func DefaultPeerStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".p2p-park-peers.json")
}

func NewPeerStore(path string) *PeerStore {
	ps := &PeerStore{
		path:  path,
		peers: make(map[string]*peerRecord),
	}
	_ = ps.load()
	return ps
}

func (ps *PeerStore) load() error {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		return nil
	}

	var recs []peerRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("peerstore decode: %w", err)
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	for i := range recs {
		r := recs[i]
		ps.peers[r.Addr] = &r
	}
	return nil
}

func (ps *PeerStore) save() error {
	ps.mu.RLock()
	recs := make([]peerRecord, 0, len(ps.peers))
	for _, r := range ps.peers {
		if r == nil {
			continue
		}
		recs = append(recs, *r)
	}
	ps.mu.RUnlock()

	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Errorf("peerstore encode: %w", err)
	}
	tmp := ps.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ps.path)
}

func (ps *PeerStore) NoteSuccess(addr string) {
	now := time.Now()

	ps.mu.Lock()
	r, ok := ps.peers[addr]
	if !ok || r == nil {
		r = &peerRecord{Addr: addr}
		ps.peers[addr] = r
	}
	r.LastSeen = now
	r.LastSuccess = now
	r.FailureCount = 0
	ps.mu.Unlock()

	_ = ps.save()
}

func (ps *PeerStore) NoteFailure(addr string) {
	now := time.Now()

	ps.mu.Lock()
	r, ok := ps.peers[addr]
	if !ok || r == nil {
		r = &peerRecord{Addr: addr}
		ps.peers[addr] = r
	}
	r.LastSeen = now
	r.FailureCount++
	ps.mu.Unlock()

	_ = ps.save()
}

func (ps *PeerStore) Candidates(maxFailures int) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	out := make([]string, 0, len(ps.peers))
	for addr, r := range ps.peers {
		if r == nil {
			continue
		}
		if r.FailureCount > maxFailures {
			continue
		}
		out = append(out, addr)
	}
	return out
}
