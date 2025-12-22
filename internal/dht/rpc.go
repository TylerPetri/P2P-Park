package dht

import (
	"context"
	"errors"
	"time"

	"p2p-park/internal/proto"
)

var (
	ErrTooManyPending = errors.New("dht: too many pending rpcs")
	ErrPeerOverloaded = errors.New("dht: peer inflight limit reached")
)

const (
	maxPendingRPCs     = 2048
	maxInflightPerPeer = 4
)

func (d *DHT) beginRPC(peerID, rpcid string, ch chan proto.DHTWire) error {
	d.pendingMu.Lock()
	if len(d.pending) >= maxPendingRPCs {
		d.pendingMu.Unlock()
		return ErrTooManyPending
	}
	d.pending[rpcid] = ch
	d.pendingMu.Unlock()

	// Bound per-peer inflight.
	d.inflightMu.Lock()
	if d.inflight[peerID] >= maxInflightPerPeer {
		d.inflightMu.Unlock()
		// undo pending
		d.pendingMu.Lock()
		delete(d.pending, rpcid)
		d.pendingMu.Unlock()
		return ErrPeerOverloaded
	}
	d.inflight[peerID]++
	d.inflightMu.Unlock()
	return nil
}

func (d *DHT) endRPC(peerID, rpcid string) {
	// pending is removed by handler on successful response; we only ensure cleanup on timeout/error.
	d.inflightMu.Lock()
	if d.inflight[peerID] > 0 {
		d.inflight[peerID]--
	}
	d.inflightMu.Unlock()
	// Ensure pending cleared (idempotent).
	d.pendingMu.Lock()
	delete(d.pending, rpcid)
	d.pendingMu.Unlock()
}

func (d *DHT) QueryFindNode(n Sender, peerID string, targetHex string, timeout time.Duration) (proto.DHTWire, error) {
	ok := false
	defer func() { d.metrics.IncRPC("FIND_NODE", ok) }()
	rpcid := newRPCID()
	ch := make(chan proto.DHTWire, 1)
	if err := d.beginRPC(peerID, rpcid, ch); err != nil {
		return proto.DHTWire{}, err
	}
	defer d.endRPC(peerID, rpcid)

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
		return proto.DHTWire{}, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-ch:
		ok = true
		return resp, nil
	case <-timer.C:
		return proto.DHTWire{}, context.DeadlineExceeded
	}
}

func (d *DHT) QueryPing(n Sender, peerID string, timeout time.Duration) (proto.DHTWire, error) {
	ok := false
	defer func() { d.metrics.IncRPC("PING", ok) }()
	rpcid := newRPCID()
	ch := make(chan proto.DHTWire, 1)
	if err := d.beginRPC(peerID, rpcid, ch); err != nil {
		return proto.DHTWire{}, err
	}
	defer d.endRPC(peerID, rpcid)

	req := proto.DHTWire{
		Kind:  "PING",
		RPCID: rpcid,
	}

	if err := n.SendToPeer(peerID, proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  n.ID(),
		Payload: proto.MustMarshal(req),
	}); err != nil {
		return proto.DHTWire{}, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-ch:
		ok = true
		return resp, nil
	case <-timer.C:
		return proto.DHTWire{}, context.DeadlineExceeded
	}
}

func (d *DHT) QueryFindValue(n Sender, peerID string, keyHex string, timeout time.Duration) (proto.DHTWire, error) {
	ok := false
	defer func() { d.metrics.IncRPC("FIND_VALUE", ok) }()
	rpcid := newRPCID()
	ch := make(chan proto.DHTWire, 1)
	if err := d.beginRPC(peerID, rpcid, ch); err != nil {
		return proto.DHTWire{}, err
	}
	defer d.endRPC(peerID, rpcid)

	req := proto.DHTWire{
		Kind:  "FIND_VALUE",
		RPCID: rpcid,
		Key:   keyHex,
	}

	if err := n.SendToPeer(peerID, proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  n.ID(),
		Payload: proto.MustMarshal(req),
	}); err != nil {
		return proto.DHTWire{}, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-ch:
		ok = true
		return resp, nil
	case <-timer.C:
		return proto.DHTWire{}, context.DeadlineExceeded
	}
}

func (d *DHT) QueryStore(n Sender, peerID string, keyHex string, rec *proto.DHTRecord, timeout time.Duration) (proto.DHTWire, error) {
	ok := false
	defer func() { d.metrics.IncRPC("STORE", ok) }()
	rpcid := newRPCID()
	ch := make(chan proto.DHTWire, 1)
	if err := d.beginRPC(peerID, rpcid, ch); err != nil {
		return proto.DHTWire{}, err
	}
	defer d.endRPC(peerID, rpcid)

	req := proto.DHTWire{
		Kind:   "STORE",
		RPCID:  rpcid,
		Key:    keyHex,
		Record: rec,
	}

	if err := n.SendToPeer(peerID, proto.Envelope{
		Type:    proto.MsgDHT,
		FromID:  n.ID(),
		Payload: proto.MustMarshal(req),
	}); err != nil {
		return proto.DHTWire{}, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-ch:
		ok = true
		return resp, nil
	case <-timer.C:
		return proto.DHTWire{}, context.DeadlineExceeded
	}
}
