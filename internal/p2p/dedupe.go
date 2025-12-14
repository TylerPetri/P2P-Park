package p2p

import (
	"sync"
	"time"
)

type seenCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]time.Time
}

func newSeenCache(ttl time.Duration) *seenCache {
	return &seenCache{
		ttl:   ttl,
		items: make(map[string]time.Time),
	}
}

// Seen returns true if id was seen recently. If not, it records it and returns false.
func (s *seenCache) Seen(id string) bool {
	if id == "" {
		return true
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	// opportunistic GC
	for k, t := range s.items {
		if now.Sub(t) > s.ttl {
			delete(s.items, k)
		}
	}

	if _, ok := s.items[id]; ok {
		return true
	}
	s.items[id] = now
	return false
}
