package dht

import (
	"context"
	"p2p-park/internal/proto"
	"time"
)

func (d *DHT) QueryFindNode(n Sender, peerID string, targetHex string, timeout time.Duration) (proto.DHTWire, error) {
	rpcid := newRPCID()

	ch := make(chan proto.DHTWire, 1)

	d.pendingMu.Lock()
	d.pending[rpcid] = ch
	d.pendingMu.Unlock()

	req := proto.DHTWire{
		Kind:   "FIND_NODE",
		RPCID:  rpcid,
		Target: targetHex,
	}

	if err := n.SendToPeer(peerID, proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  n.ID(),
		Payload: proto.MustMarshal(req),
	}); err != nil {
		d.pendingMu.Lock()
		delete(d.pending, rpcid)
		d.pendingMu.Unlock()
		return proto.DHTWire{}, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-ch:
		return resp, nil
	case <-timer.C:
		d.pendingMu.Lock()
		delete(d.pending, rpcid)
		d.pendingMu.Unlock()
		return proto.DHTWire{}, context.DeadlineExceeded
	}
}
