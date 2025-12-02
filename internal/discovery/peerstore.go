package discovery

import (
	"encoding/json"
	"os"
	"p2p-park/internal/p2p"
	"path/filepath"
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
	peers map[string]*peerRecord
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
		return err
	}
	var recs []*peerRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return err
	}
	for _, r := range recs {
		ps.peers[r.Addr] = r
	}
	return nil
}

func (ps *PeerStore) save() error {
	var recs []*peerRecord
	for _, r := range ps.peers {
		recs = append(recs, r)
	}
	tmp := ps.path + ".tmp"
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ps.path)
}

func (ps *PeerStore) NoteSuccess(addr string) {
	r, ok := ps.peers[addr]
	if !ok {
		r = &peerRecord{Addr: addr}
		ps.peers[addr] = r
	}
	now := time.Now()
	r.LastSeen = now
	r.LastSuccess = now
	r.FailureCount = 0
	_ = ps.save()
}

func (ps *PeerStore) NoteFailure(addr string) {
	r, ok := ps.peers[addr]
	if !ok {
		r = &peerRecord{Addr: addr}
		ps.peers[addr] = r
	}
	r.FailureCount++
	r.LastSeen = time.Now()
	_ = ps.save()
}

type PeerStoreStrategy struct {
	Store *PeerStore
}

func (s *PeerStoreStrategy) Discover(_ *p2p.Node) []string {
	var out []string
	for addr, r := range s.Store.peers {
		if r.FailureCount > 5 {
			continue
		}
		out = append(out, addr)
	}
	return out
}

func DefaultPeerStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".p2p-park-peers.json")
}
