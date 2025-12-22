package dht

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type NodeInfo struct {
	NodeID    NodeID
	NodeIDHex string

	PeerID string // hex(pubkey) for transport dialing

	Addr     string
	Name     string
	LastSeen time.Time
}

type bucket struct {
	nodes []NodeInfo // LRU: index 0 = most recently seen; end = least
	repl  []NodeInfo // replacement cache (bounded)
}

type DiversityPolicy struct {
	MaxPerSubnet int
}

type RoutingTable struct {
	self NodeID
	k    int

	mu      sync.RWMutex
	buckets [256]bucket

	diversity DiversityPolicy
}

func NewRoutingTable(self NodeID, k int) *RoutingTable {
	if k <= 0 {
		k = 20
	}
	return &RoutingTable{self: self, k: k, diversity: DiversityPolicy{MaxPerSubnet: 2}}
}

// Upsert is a "no-network" upsert: it maintains LRU ordering.
// If a bucket is full, it DOES NOT evict; it will just drop the new node.
// (Network-aware eviction is done via UpsertWithEviction below.)
func (rt *RoutingTable) Upsert(nodeID NodeID, peerID, addr, name string) {
	rt.upsertLRU(nodeID, peerID, addr, name, time.Now(), nil)
}

// PingFunc returns true if the node is alive.
type PingFunc func(NodeInfo) bool

// UpsertWithEviction implements Kademlia bucket semantics:
// - If node exists: move-to-front
// - Else if space: insert at front
// - Else ping LRU tail: if dead -> evict tail, insert new; if alive -> keep tail, add new to replacement cache.
func (rt *RoutingTable) UpsertWithEviction(nodeID NodeID, peerID, addr, name string, ping PingFunc) {
	rt.upsertLRU(nodeID, peerID, addr, name, time.Now(), ping)
}

func (rt *RoutingTable) upsertLRU(id NodeID, peerID, addr, name string, now time.Time, ping PingFunc) {
	if id == rt.self {
		return
	}
	bi := BucketIndex(rt.self, id)
	if bi < 0 || bi >= 256 {
		return
	}

	rt.mu.Lock()
	b := rt.buckets[bi]

	for i := range b.nodes {
		if b.nodes[i].NodeID == id {
			ni := b.nodes[i]
			if peerID != "" {
				ni.PeerID = peerID
			}
			ni.Addr = addr
			if name != "" {
				ni.Name = name
			}
			ni.LastSeen = now

			copy(b.nodes[i:], b.nodes[i+1:])
			b.nodes = b.nodes[:len(b.nodes)-1]
			b.nodes = append([]NodeInfo{ni}, b.nodes...)

			rt.buckets[bi] = b
			rt.mu.Unlock()
			return
		}
	}

	ni := NodeInfo{
		NodeID:    id,
		NodeIDHex: id.Hex(),
		PeerID:    peerID,
		Addr:      addr,
		Name:      name,
		LastSeen:  now,
	}

	// Anti-eclipse diversity: cap number of nodes from the same subnet per bucket.
	maxPerSubnet := rt.diversity.MaxPerSubnet
	if maxPerSubnet > 0 {
		sk := subnetKey(ni.Addr)
		if sk != "" {
			cnt := 0
			for i := range b.nodes {
				if subnetKey(b.nodes[i].Addr) == sk {
					cnt++
				}
			}
			if cnt >= maxPerSubnet {
				rt.mu.Unlock()
				return
			}
		}

	}
	// Space available => insert at front
	if len(b.nodes) < rt.k {
		b.nodes = append([]NodeInfo{ni}, b.nodes...)
		rt.buckets[bi] = b
		rt.mu.Unlock()
		return
	}

	// Bucket full:
	// If no ping func, we cannot safely evict; drop new node.
	if ping == nil {
		rt.mu.Unlock()
		return
	}

	// Ping LRU tail outside lock to avoid blocking the entire table.
	tail := b.nodes[len(b.nodes)-1]
	rt.mu.Unlock()

	alive := ping(tail)

	rt.mu.Lock()
	b = rt.buckets[bi]

	if len(b.nodes) < rt.k {
		b.nodes = append([]NodeInfo{ni}, b.nodes...)
		rt.buckets[bi] = b
		rt.mu.Unlock()
		return
	}

	// Re-identify tail (could have changed)
	curTail := b.nodes[len(b.nodes)-1]

	if alive && curTail.NodeID == tail.NodeID {
		// Keep tail, drop new from main list, but keep as replacement
		b = rt.addReplacement(b, ni)
		rt.buckets[bi] = b
		rt.mu.Unlock()
		return
	}

	// Tail considered dead => evict it (best-effort)
	b.nodes = b.nodes[:len(b.nodes)-1]
	b.nodes = append([]NodeInfo{ni}, b.nodes...)
	rt.buckets[bi] = b
	rt.mu.Unlock()
}

func (rt *RoutingTable) addReplacement(b bucket, ni NodeInfo) bucket {
	const replMax = 10
	for i := range b.repl {
		if b.repl[i].NodeID == ni.NodeID {
			return b
		}
	}
	b.repl = append([]NodeInfo{ni}, b.repl...)
	if len(b.repl) > replMax {
		b.repl = b.repl[:replMax]
	}
	return b
}

func (rt *RoutingTable) Closest(target NodeID, n int) []NodeInfo {
	if n <= 0 {
		n = rt.k
	}

	rt.mu.RLock()
	defer rt.mu.RUnlock()

	// Collect candidates (bounded-ish; table size is small enough)
	all := make([]NodeInfo, 0, 256*rt.k)
	for i := 0; i < 256; i++ {
		all = append(all, rt.buckets[i].nodes...)
	}

	SortByDistance(all, target)

	if len(all) > n {
		all = all[:n]
	}
	return all
}

// SortByDistance sorts NodeInfo slice by XOR distance to target.
func SortByDistance(nodes []NodeInfo, target NodeID) {
	type nd struct {
		ni   NodeInfo
		dist NodeID
	}
	tmp := make([]nd, len(nodes))
	for i := range nodes {
		tmp[i] = nd{ni: nodes[i], dist: Distance(nodes[i].NodeID, target)}
	}

	for i := 1; i < len(tmp); i++ {
		j := i
		for j > 0 && DistanceLess(tmp[j].dist, tmp[j-1].dist) {
			tmp[j], tmp[j-1] = tmp[j-1], tmp[j]
			j--
		}
	}
	for i := range tmp {
		nodes[i] = tmp[i].ni
	}
}

func subnetKey(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = ""
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "dns:" + strings.ToLower(host)
	}

	if ip.IsLoopback() {
		if port != "" {
			return "loopback:" + host + ":" + port
		}
		return "loopback:" + host
	}

	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("v4:%d.%d.%d.0/24", v4[0], v4[1], v4[2])
	}

	ip = ip.To16()
	if ip == nil {
		return "ip:unknown"
	}

	pfx := make(net.IP, 16)
	copy(pfx, ip)
	for i := 8; i < 16; i++ {
		pfx[i] = 0
	}
	return "v6:" + pfx.String() + "/64"
}

// Size returns total number of nodes in the routing table.
func (rt *RoutingTable) Size() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	n := 0
	for i := 0; i < 256; i++ {
		n += len(rt.buckets[i].nodes)
	}
	return n
}

// BucketSize returns number of nodes in a bucket.
func (rt *RoutingTable) BucketSize(bucket int) int {
	if bucket < 0 || bucket >= 256 {
		return 0
	}
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.buckets[bucket].nodes)
}

func WithDiversityPolicy(p DiversityPolicy) Option {
	return func(d *DHT) { d.rt.SetDiversityLimit(p.MaxPerSubnet) }
}

func (rt *RoutingTable) SetDiversityLimit(maxPerSubnet int) {
	rt.mu.Lock()
	rt.diversity.MaxPerSubnet = maxPerSubnet
	rt.mu.Unlock()
}
