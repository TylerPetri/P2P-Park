package dht

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func randID(t *testing.T) NodeID {
	t.Helper()
	var id NodeID
	_, err := rand.Read(id[:])
	if err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return id
}

func xorBytes(a, b NodeID) [32]byte {
	return Xor(a, b)
}

func TestXorSymmetry(t *testing.T) {
	a := randID(t)
	b := randID(t)
	if Xor(a, b) != Xor(b, a) {
		t.Fatalf("xor not symmetric")
	}
}

func TestBucketIndex_MSB(t *testing.T) {
	var self NodeID
	var peer NodeID
	peer[0] = 0x80 // differs at the very first bit
	if got := BucketIndex(self, peer); got != 0 {
		t.Fatalf("expected bucket index 0, got %d", got)
	}
}

func TestBucketIndex_Identical(t *testing.T) {
	id := randID(t)
	if got := BucketIndex(id, id); got != -1 {
		t.Fatalf("expected -1 for identical ids, got %d", got)
	}
}

func TestRoutingTable_ClosestSortedByDistance(t *testing.T) {
	self := randID(t)
	rt := NewRoutingTable(self, 8)

	target := randID(t)

	// Insert enough random peers.
	for i := 0; i < 50; i++ {
		id := randID(t)
		rt.Upsert(id, randID(t).Hex(), "127.0.0.1:1234", "p")
	}

	got := rt.Closest(target, 10)
	if len(got) == 0 {
		t.Fatalf("expected some closest nodes")
	}
	if len(got) > 10 {
		t.Fatalf("expected <=10, got %d", len(got))
	}

	// Verify sorted ascending by XOR distance to target.
	for i := 1; i < len(got); i++ {
		prev := xorBytes(got[i-1].NodeID, target)
		cur := xorBytes(got[i].NodeID, target)
		if bytes.Compare(prev[:], cur[:]) > 0 {
			t.Fatalf("closest not sorted at i=%d", i)
		}
	}
}
