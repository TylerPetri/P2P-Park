package points

import (
	"crypto/ed25519"
	"p2p-park/internal/proto"
	"sort"
	"sync"
)

// Engine tracks scores for ourselves and others.
// It uses a simple "last higher Version wins" merge.
type Engine struct {
	mu sync.RWMutex

	selfID   string
	selfName string

	selfPoints  int64
	selfVersion uint64

	selfPriv ed25519.PrivateKey
	selfPub  ed25519.PublicKey

	others map[string]proto.PointsSnapshot
}

// NewEngine initializes a points engine for a given local identity.
func NewEngine(selfID, selfName string, priv ed25519.PrivateKey, pub ed25519.PublicKey) *Engine {
	return &Engine{
		selfID:   selfID,
		selfName: selfName,
		selfPriv: priv,
		selfPub:  pub,
		others:   make(map[string]proto.PointsSnapshot),
	}
}

// signSnapshot creates a SignedPointsSnapshot for a local snapshot.
func (e *Engine) signSnapshot(snap proto.PointsSnapshot) (proto.SignedPointsSnapshot, error) {
	data, err := proto.EncodeSnapshotCanonical(snap)
	if err != nil {
		return proto.SignedPointsSnapshot{}, err
	}
	sig := ed25519.Sign(e.selfPriv, data)
	return proto.SignedPointsSnapshot{
		Snapshot:  snap,
		PubKey:    e.selfPub,
		Signature: sig,
	}, nil
}

// verifySigned verifies that sig(pub) matches snapshot.
func verifySigned(s proto.SignedPointsSnapshot) bool {
	data, err := proto.EncodeSnapshotCanonical(s.Snapshot)
	if err != nil {
		return false
	}
	if len(s.PubKey) != ed25519.PublicKeySize {
		return false
	}
	pub := ed25519.PublicKey(s.PubKey)
	return ed25519.Verify(pub, data, s.Signature)
}

// AddSelf increments our own points by delta and returns a signed snapshot.
//
// This is where we bump the Version so remote nodes can do last-write-wins.
func (e *Engine) AddSelf(delta int64) (proto.SignedPointsSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.selfVersion++
	e.selfPoints += delta

	snap := proto.PointsSnapshot{
		PlayerID: e.selfID,
		Name:     e.selfName,
		Points:   e.selfPoints,
		Version:  e.selfVersion,
	}
	return e.signSnapshot(snap)
}

// SnapshotSelf returns the current signed self snapshot.
func (e *Engine) SnapshotSelf() (proto.SignedPointsSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snap := proto.PointsSnapshot{
		PlayerID: e.selfID,
		Name:     e.selfName,
		Points:   e.selfPoints,
		Version:  e.selfVersion,
	}
	return e.signSnapshot(snap)
}

// ApplyRemote merges a remote signed snapshot into our view.
// Returns true if it changed our local view, false if ignored (older or same version).
func (e *Engine) ApplyRemote(s proto.SignedPointsSnapshot) bool {
	if !verifySigned(s) {
		return false
	}

	snap := s.Snapshot

	e.mu.Lock()
	defer e.mu.Unlock()

	if snap.PlayerID == e.selfID {
		return false
	}

	cur, ok := e.others[snap.PlayerID]
	if ok && snap.Version <= cur.Version {
		return false
	}

	e.others[snap.PlayerID] = snap
	return true
}

// All returns a slice of *unsigned* score snapshots including ourselves and others, sorted by points descending.
func (e *Engine) All() []proto.PointsSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]proto.PointsSnapshot, 0, len(e.others)+1)

	out = append(out, proto.PointsSnapshot{
		PlayerID: e.selfID,
		Name:     e.selfName,
		Points:   e.selfPoints,
		Version:  e.selfVersion,
	})

	for _, s := range e.others {
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Points == out[j].Points {
			return out[i].PlayerID < out[j].PlayerID
		}
		return out[i].Points > out[j].Points
	})

	return out
}
