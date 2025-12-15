package dht

import (
	"sort"
	"sync"
	"time"
)

type NodeInfo struct {
	ID       NodeID
	IDHex    string // cached
	Addr     string
	Name     string
	LastSeen time.Time
}

type RoutingTable struct {
	self NodeID
	k    int

	mu      sync.RWMutex
	buckets [256][]NodeInfo // simple slice buckets for Phase 1
}

func NewRoutingTable(self NodeID, k int) *RoutingTable {
	if k <= 0 {
		k = 20
	}
	return &RoutingTable{self: self, k: k}
}

func (rt *RoutingTable) Upsert(id NodeID, addr, name string) {
	if id == rt.self {
		return
	}
	bi := BucketIndex(rt.self, id)
	if bi < 0 || bi >= 256 {
		return
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	b := rt.buckets[bi]

	// update existing
	for i := range b {
		if b[i].ID == id {
			b[i].Addr = addr
			if name != "" {
				b[i].Name = name
			}
			b[i].LastSeen = time.Now()
			rt.buckets[bi] = b
			return
		}
	}

	// append
	ni := NodeInfo{
		ID:       id,
		IDHex:    id.Hex(),
		Addr:     addr,
		Name:     name,
		LastSeen: time.Now(),
	}
	b = append(b, ni)

	// trim (Phase 1: drop oldest; later: LRU + ping eviction)
	if len(b) > rt.k {
		sort.Slice(b, func(i, j int) bool { return b[i].LastSeen.After(b[j].LastSeen) })
		b = b[:rt.k]
	}

	rt.buckets[bi] = b
}

func (rt *RoutingTable) Closest(target NodeID, n int) []NodeInfo {
	if n <= 0 {
		n = rt.k
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	all := make([]NodeInfo, 0, 256*rt.k)
	for i := 0; i < 256; i++ {
		all = append(all, rt.buckets[i]...)
	}

	sort.Slice(all, func(i, j int) bool {
		di := Xor(all[i].ID, target)
		dj := Xor(all[j].ID, target)
		return di.Hex() < dj.Hex() // cheap lex compare of XOR bytes
	})

	if len(all) > n {
		all = all[:n]
	}
	return all
}
