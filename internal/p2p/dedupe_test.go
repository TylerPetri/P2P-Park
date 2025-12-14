package p2p

import (
	"testing"
	"time"
)

func TestSeenCache(t *testing.T) {
	s := newSeenCache(50 * time.Millisecond)
	if s.Seen("x") {
		t.Fatalf("first time should be unseen")
	}
	if !s.Seen("x") {
		t.Fatalf("second time should be seen")
	}
	time.Sleep(60 * time.Millisecond)
	if s.Seen("x") {
		t.Fatalf("after ttl it should expire and be unseen")
	}
}
