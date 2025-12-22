package dht

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func benchPeerID(b *testing.B) string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func BenchmarkRouting_UpsertWithEviction_NoPing(b *testing.B) {
	peer := benchPeerID(b)
	self, _ := NodeIDFromPeerID(peer)
	rt := NewRoutingTable(self, 20)

	ids := make([]NodeID, 2000)
	pids := make([]string, 2000)
	for i := range ids {
		pids[i] = benchPeerID(b)
		ids[i], _ = NodeIDFromPeerID(pids[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := i % len(ids)
		rt.Upsert(ids[j], pids[j], "127.0.0.1:0", "n")
	}
}

func BenchmarkRouting_Closest(b *testing.B) {
	peer := benchPeerID(b)
	self, _ := NodeIDFromPeerID(peer)
	rt := NewRoutingTable(self, 20)

	for i := 0; i < 5000; i++ {
		pid := benchPeerID(b)
		id, _ := NodeIDFromPeerID(pid)
		rt.Upsert(id, pid, "127.0.0.1:0", "n")
	}

	target := RandomNodeID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rt.Closest(target, 20)
	}
}
