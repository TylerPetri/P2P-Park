package dht

import (
	"encoding/json"
	"log"
	"testing"

	"p2p-park/internal/proto"
)

type fakeSender struct {
	selfID string

	sentTo  string
	sentEnv proto.Envelope

	logf func(string, ...any)
}

func (f *fakeSender) ID() string { return f.selfID }

func (f *fakeSender) SendToPeer(id string, env proto.Envelope) error {
	f.sentTo = id
	f.sentEnv = env
	return nil
}

func (f *fakeSender) Logf(format string, args ...any) {
	if f.logf != nil {
		f.logf(format, args...)
		return
	}
	_ = log.New(log.Writer(), "", 0) // do nothing by default
}

func TestHandler_PingPong(t *testing.T) {
	selfPeerID := MustParseNodeIDHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f").Hex()

	h, err := New(selfPeerID)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	n := &fakeSender{selfID: selfPeerID}

	req := proto.DHTWire{
		Kind:  "PING",
		RPCID: "rpc-1",
	}
	env := proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Payload: proto.MustMarshal(req),
	}

	h.HandleDHT(n, env.FromID, "127.0.0.1:9999", "peerA", env)

	if n.sentTo != env.FromID {
		t.Fatalf("expected reply to %s, got %s", env.FromID, n.sentTo)
	}
	if n.sentEnv.Type != proto.MsgDHT {
		t.Fatalf("expected MsgDHT reply, got %s", n.sentEnv.Type)
	}

	var got proto.DHTWire
	if err := json.Unmarshal(n.sentEnv.Payload, &got); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}
	if got.Kind != "PONG" {
		t.Fatalf("expected PONG, got %s", got.Kind)
	}
	if got.RPCID != "rpc-1" {
		t.Fatalf("expected same RPCID, got %s", got.RPCID)
	}
}

func TestHandler_FindNode_ReturnsClosest(t *testing.T) {
	selfPeerID := MustParseNodeIDHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f").Hex()

	h, err := New(selfPeerID)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	// Seed routing table with known nodes.
	peer1 := MustParseNodeIDHex("1111111111111111111111111111111111111111111111111111111111111111").Hex()
	peer2 := MustParseNodeIDHex("2222222222222222222222222222222222222222222222222222222222222222").Hex()
	peer3 := MustParseNodeIDHex("3333333333333333333333333333333333333333333333333333333333333333").Hex()

	id1, _ := NodeIDFromPeerID(peer1)
	id2, _ := NodeIDFromPeerID(peer2)
	id3, _ := NodeIDFromPeerID(peer3)

	h.rt.Upsert(id1, peer1, "10.0.0.1:1001", "n1")
	h.rt.Upsert(id2, peer2, "10.0.0.2:1002", "n2")
	h.rt.Upsert(id3, peer3, "10.0.0.3:1003", "n3")

	n := &fakeSender{selfID: selfPeerID}

	from := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	target := id2.Hex()

	req := proto.DHTWire{
		Kind:   "FIND_NODE",
		RPCID:  "rpc-2",
		Target: target,
	}

	env := proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  from,
		Payload: proto.MustMarshal(req),
	}

	h.HandleDHT(n, from, "127.0.0.1:9999", "peerA", env)

	var got proto.DHTWire
	if err := json.Unmarshal(n.sentEnv.Payload, &got); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}

	if got.Kind != "NODES" {
		t.Fatalf("expected NODES, got %s", got.Kind)
	}
	if got.RPCID != "rpc-2" {
		t.Fatalf("expected same RPCID, got %s", got.RPCID)
	}
	if got.Target != target {
		t.Fatalf("expected same target, got %s", got.Target)
	}
	if len(got.Nodes) == 0 {
		t.Fatalf("expected some nodes in response")
	}

	// Ensure the target itself is present (since we inserted it).
	found := false
	for _, nd := range got.Nodes {
		if nd.NodeID == target {
			found = true
			if nd.Addr != "10.0.0.2:1002" {
				t.Fatalf("expected addr for target node, got %s", nd.Addr)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected response to include target node %s", target)
	}
}
