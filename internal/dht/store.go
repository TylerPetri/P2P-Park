package dht

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type nodeRecord struct {
	NodeID       string    `json:"node_id"` // hex NodeID
	Addr         string    `json:"addr"`
	Name         string    `json:"name,omitempty"`
	LastSeen     time.Time `json:"last_seen"`
	LastSuccess  time.Time `json:"last_success"`
	FailureCount int       `json:"failures"`
}

type Store struct {
	path  string
	mu    sync.RWMutex
	nodes map[string]*nodeRecord // key: NodeID
}

func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".p2p-park-dht.json")
}

func NewStore(path string) *Store {
	if path == "" {
		path = DefaultStorePath()
	}
	s := &Store{
		path:  path,
		nodes: make(map[string]*nodeRecord),
	}
	_ = s.load()
	return s
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	var recs []nodeRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("dhtstore decode: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range recs {
		r := recs[i]
		if r.NodeID == "" || r.Addr == "" {
			continue
		}
		s.nodes[r.NodeID] = &r
	}
	return nil
}

func (s *Store) save() error {
	s.mu.RLock()
	recs := make([]nodeRecord, 0, len(s.nodes))
	for _, r := range s.nodes {
		if r == nil || r.NodeID == "" || r.Addr == "" {
			continue
		}
		recs = append(recs, *r)
	}
	s.mu.RUnlock()

	// Stable output helps with diffs + sanity.
	sort.Slice(recs, func(i, j int) bool { return recs[i].NodeID < recs[j].NodeID })

	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Errorf("dhtstore encode: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) NoteSuccess(nodeID, addr, name string) {
	now := time.Now()

	s.mu.Lock()
	r := s.nodes[nodeID]
	if r == nil {
		r = &nodeRecord{NodeID: nodeID}
		s.nodes[nodeID] = r
	}
	r.Addr = addr
	r.Name = name
	r.LastSeen = now
	r.LastSuccess = now
	r.FailureCount = 0
	s.mu.Unlock()

	_ = s.save()
}

func (s *Store) NoteFailure(nodeID string) {
	now := time.Now()

	s.mu.Lock()
	r := s.nodes[nodeID]
	if r == nil {
		r = &nodeRecord{NodeID: nodeID}
		s.nodes[nodeID] = r
	}
	r.LastSeen = now
	r.FailureCount++
	s.mu.Unlock()

	_ = s.save()
}

// Candidates returns best addresses to try first.
func (s *Store) Candidates(maxFailures int, limit int) []string {
	s.mu.RLock()
	type cand struct {
		addr        string
		lastSuccess time.Time
		fail        int
	}
	cs := make([]cand, 0, len(s.nodes))
	for _, r := range s.nodes {
		if r == nil || r.Addr == "" || r.NodeID == "" {
			continue
		}
		if r.FailureCount > maxFailures {
			continue
		}
		cs = append(cs, cand{addr: r.Addr, lastSuccess: r.LastSuccess, fail: r.FailureCount})
	}
	s.mu.RUnlock()

	sort.Slice(cs, func(i, j int) bool {
		if !cs[i].lastSuccess.Equal(cs[j].lastSuccess) {
			return cs[i].lastSuccess.After(cs[j].lastSuccess)
		}
		return cs[i].fail < cs[j].fail
	})

	if limit > 0 && len(cs) > limit {
		cs = cs[:limit]
	}
	out := make([]string, 0, len(cs))
	seen := make(map[string]struct{}, len(cs))
	for _, c := range cs {
		if c.addr == "" {
			continue
		}
		if _, ok := seen[c.addr]; ok {
			continue
		}
		seen[c.addr] = struct{}{}
		out = append(out, c.addr)
	}
	return out
}
