package points

import (
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

	others map[string]proto.PointsSnapshot
}

// NewEngine initializes a points engine for a given local identity.
func NewEngine(selfID, selfName string) *Engine {
	return &Engine{
		selfID:   selfID,
		selfName: selfName,
		others:   make(map[string]proto.PointsSnapshot),
	}
}

// AddSelf increments our own points by delta and returns the new snapshot.
//
// This is where we bump the Version so remote nodes can do last-write-wins.
func (e *Engine) AddSelf(delta int64) proto.PointsSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.selfVersion++
	e.selfPoints += delta

	return proto.PointsSnapshot{
		PlayerID: e.selfID,
		Name:     e.selfName,
		Points:   e.selfPoints,
		Version:  e.selfVersion,
	}
}

// SnapshotSelf returns our current local score snapshot.
func (e *Engine) SnapshotSelf() proto.PointsSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return proto.PointsSnapshot{
		PlayerID: e.selfID,
		Name:     e.selfName,
		Points:   e.selfPoints,
		Version:  e.selfVersion,
	}
}

// ApplyRemote merges a remote snapshot into our view.
// Returns true if it changed our local view, false if ignored (older or same version).
func (e *Engine) ApplyRemote(s proto.PointsSnapshot) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if s.PlayerID == e.selfID {
		return false
	}

	cur, ok := e.others[s.PlayerID]
	if ok && s.Version <= cur.Version {
		return false
	}

	e.others[s.PlayerID] = s
	return true
}

// All returns a slice of score snapshots including ourselves and others, sorted by points descending.
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
