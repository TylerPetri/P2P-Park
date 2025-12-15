package dht

import (
	"encoding/json"

	"p2p-park/internal/proto"
)

func (d *DHT) HandleDHT(n Sender, fromPeerID string, fromAddr string, fromName string, env proto.Envelope) {
	var w proto.DHTWire
	if err := json.Unmarshal(env.Payload, &w); err != nil {
		n.Logf("dht: bad payload from %s: %v", fromPeerID, err)
		return
	}

	// Update routing table on any DHT traffic.
	d.OnPeerSeen(fromPeerID, fromAddr, fromName)

	// Deliver responses to pending RPC waiters.
	if w.RPCID != "" && (w.Kind == "NODES" || w.Kind == "PONG") {
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
				ID:   ni.IDHex,
				Addr: ni.Addr,
				Name: ni.Name,
			})
		}

		reply := proto.DHTWire{Kind: "NODES", RPCID: w.RPCID, Target: w.Target, Nodes: out}
		_ = n.SendToPeer(fromPeerID, proto.Envelope{
			Type:    proto.MsgDHT,
			FromID:  n.ID(),
			Payload: proto.MustMarshal(reply),
		})

	default:
		// ignore unknown kinds for forward-compat
		return
	}
}
