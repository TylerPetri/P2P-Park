package dht

import (
	"encoding/json"
	"time"

	"p2p-park/internal/proto"
)

func (d *DHT) HandleDHT(n Sender, fromPeerID string, fromAddr string, fromName string, env proto.Envelope) {
	var w proto.DHTWire
	if err := json.Unmarshal(env.Payload, &w); err != nil {
		n.Logf("dht: bad payload from %s: %v", fromPeerID, err)
		return
	}

	now := time.Now()
	d.rlMu.Lock()
	b := d.rl[fromPeerID]
	if b == nil {
		b = &tokenBucket{}
		d.rl[fromPeerID] = b
	}
	ok := b.allow(now, 20 /* req/sec */, 40 /* burst */, 1 /* cost */)
	d.rlMu.Unlock()
	if !ok {
		return
	}

	// Update routing table on any DHT traffic.
	d.ObservePeer(n, fromPeerID, fromAddr, fromName)

	// Deliver responses to pending RPC waiters.
	if w.RPCID != "" && (w.Kind == "NODES" || w.Kind == "PONG" || w.Kind == "VALUE" || w.Kind == "STORE_RESULT") {
		d.pendingMu.Lock()
		ch := d.pending[w.RPCID]
		if ch != nil {
			delete(d.pending, w.RPCID)
		}
		d.pendingMu.Unlock()

		if ch != nil {
			select {
			case ch <- w:
			default:
			}
			return
		}
	}

	switch w.Kind {
	case "PING":
		reply := proto.DHTWire{Kind: "PONG", RPCID: w.RPCID}
		_ = n.SendToPeer(fromPeerID, proto.Envelope{
			Type:    proto.MsgDHT,
			FromID:  n.ID(),
			Payload: proto.MustMarshal(reply),
		})

	case "FIND_NODE":
		target, err := ParseNodeIDHex(w.Target)
		if err != nil {
			return
		}

		closest := d.rt.Closest(target, 20)
		out := make([]proto.DHTNode, 0, len(closest))
		for _, ni := range closest {
			out = append(out, proto.DHTNode{
				NodeID: ni.NodeIDHex,
				PeerID: ni.PeerID,
				Addr:   ni.Addr,
				Name:   ni.Name,
			})
		}

		reply := proto.DHTWire{Kind: "NODES", RPCID: w.RPCID, Target: w.Target, Nodes: out}
		_ = n.SendToPeer(fromPeerID, proto.Envelope{
			Type:    proto.MsgDHT,
			FromID:  n.ID(),
			Payload: proto.MustMarshal(reply),
		})

	case "STORE":
		// Validate minimal fields
		key, err := ParseKeyHex(w.Key)
		if err != nil || w.Record == nil {
			reply := proto.DHTWire{Kind: "STORE_RESULT", RPCID: w.RPCID, OK: false, Error: "bad_request"}
			_ = n.SendToPeer(fromPeerID, proto.Envelope{
				Type:    proto.MsgDHT,
				FromID:  n.ID(),
				Payload: proto.MustMarshal(reply),
			})
			return
		}

		if err := d.ValidateRecordAgainstKey(key, w.Record); err != nil {
			reply := proto.DHTWire{Kind: "STORE_RESULT", RPCID: w.RPCID, OK: false, Error: err.Error()}
			_ = n.SendToPeer(fromPeerID, proto.Envelope{
				Type:    proto.MsgDHT,
				FromID:  n.ID(),
				Payload: proto.MustMarshal(reply),
			})
			return
		}

		err = d.rs.Put(key, w.Record, time.Now())
		ok := err == nil
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		reply := proto.DHTWire{Kind: "STORE_RESULT", RPCID: w.RPCID, OK: ok, Error: errMsg}
		_ = n.SendToPeer(fromPeerID, proto.Envelope{
			Type:    proto.MsgDHT,
			FromID:  n.ID(),
			Payload: proto.MustMarshal(reply),
		})

	case "FIND_VALUE":
		key, err := ParseKeyHex(w.Key)
		if err != nil {
			return
		}

		if rec, ok := d.rs.Get(key, time.Now()); ok {
			if d.ValidateRecordAgainstKey(key, rec) == nil {
				reply := proto.DHTWire{Kind: "VALUE", RPCID: w.RPCID, Key: w.Key, Record: rec}
				_ = n.SendToPeer(fromPeerID, proto.Envelope{
					Type:    proto.MsgDHT,
					FromID:  n.ID(),
					Payload: proto.MustMarshal(reply),
				})
				return
			}
		}

		// Not found => return closest nodes (Kademlia behavior)
		target, _ := ParseNodeIDHex(w.Key)
		closest := d.rt.Closest(target, 20)
		out := make([]proto.DHTNode, 0, len(closest))
		for _, ni := range closest {
			out = append(out, proto.DHTNode{
				NodeID: ni.NodeIDHex,
				PeerID: ni.PeerID,
				Addr:   ni.Addr,
				Name:   ni.Name,
			})
		}

		reply := proto.DHTWire{Kind: "VALUE", RPCID: w.RPCID, Key: w.Key, Nodes: out}
		_ = n.SendToPeer(fromPeerID, proto.Envelope{
			Type:    proto.MsgDHT,
			FromID:  n.ID(),
			Payload: proto.MustMarshal(reply),
		})

	default:
		return
	}
}
