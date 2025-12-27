package grants

import (
	"sort"
	"sync"
	"time"

	"p2p-park/internal/proto"
)

// Award is a computed score entry.
type Award struct {
	PlayerID string
	Name     string
	Points   int64
	Updated  time.Time
}

// Ledger tracks points awarded by signed grants.
// Points only move when a valid QuizGrant is observed.
type Ledger struct {
	mu sync.RWMutex

	// grants is a de-dup set by GrantID
	grants map[string]proto.QuizGrant
	// totals is the sum of grants per recipient
	totals map[string]int64
	// names is best-effort display names, learned from opens and peers
	names map[string]string
}

func NewLedger() *Ledger {
	return &Ledger{
		grants: make(map[string]proto.QuizGrant),
		totals: make(map[string]int64),
		names:  make(map[string]string),
	}
}

func (l *Ledger) NoteName(peerID, name string) {
	if peerID == "" || name == "" {
		return
	}
	l.mu.Lock()
	if _, ok := l.names[peerID]; !ok {
		l.names[peerID] = name
	}
	l.mu.Unlock()
}

// ApplyGrant verifies and applies a grant.
// Returns true if it was new and changed totals.
func (l *Ledger) ApplyGrant(g proto.QuizGrant) bool {
	if err := VerifyGrant(g); err != nil {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.grants[g.GrantID]; ok {
		return false
	}
	l.grants[g.GrantID] = g
	l.totals[g.RecipientID] += g.Points
	return true
}

func (l *Ledger) Total(peerID string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.totals[peerID]
}

func (l *Ledger) Leaderboard() []Award {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Award, 0, len(l.totals))
	for id, pts := range l.totals {
		out = append(out, Award{PlayerID: id, Name: l.names[id], Points: pts})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Points == out[j].Points {
			return out[i].PlayerID < out[j].PlayerID
		}
		return out[i].Points > out[j].Points
	})
	return out
}
