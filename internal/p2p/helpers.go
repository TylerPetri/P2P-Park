package p2p

import (
	"context"
	"fmt"
	"p2p-park/internal/proto"
)

func (p *peer) writeLoop(ctx context.Context, n *Node) {
	for {
		select {
		case env, ok := <-p.sendCh:
			if !ok {
				return
			}
			if err := p.writer.Encode(env); err != nil {
				n.logf("write to %s failed: %v", p.id, err)
				go n.removePeer(p.id)
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// PeerCount returns the current number of connected peers.
func (n *Node) PeerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.peers)
}

// PeerIDs returns a snapshot of current peer IDs.
func (n *Node) PeerIDs() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	ids := make([]string, 0, len(n.peers))
	for id := range n.peers {
		ids = append(ids, id)
	}
	return ids
}

func (n *Node) sendAsync(p *peer, env proto.Envelope) {
	select {
	case p.sendCh <- env:
		// queued
	default:
		n.logf("peer %s send buffer full, dropping", p.id)
		go n.removePeer(p.id)
	}
}

// SendToPeer sends an envelope to a peer by network ID.
// It returns an error if the peer is not known.
func (n *Node) SendToPeer(id string, env proto.Envelope) error {
	n.mu.RLock()
	p, ok := n.peers[id]
	n.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown peer %q", id)
	}

	n.sendAsync(p, env)
	return nil
}

func (n *Node) sendIdentify(p *peer) error {
	id := n.id

	ident := proto.Identify{
		Name:    n.cfg.Name,
		UserPub: id.SignPub,
	}

	env := proto.Envelope{
		Type:    proto.MsgIdentify,
		FromID:  n.id.ID, // network ID (Noise hex)
		Payload: proto.MustMarshal(ident),
	}
	n.sendAsync(p, env)
	return nil
}

func (n *Node) PeerDisplayName(id string) string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	p, ok := n.peers[id]
	if !ok || p == nil {
		if len(id) > 8 {
			return id[:8]
		}
		return id
	}

	if p.name != "" {
		return p.name
	}

	if p.userID != "" {
		if len(p.userID) > 8 {
			return p.userID[:8]
		}
		return p.userID
	}

	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func (n *Node) SnapshotPeers() []PeerSnapshot {
	n.mu.RLock()
	defer n.mu.RUnlock()

	out := make([]PeerSnapshot, 0, len(n.peers))
	for _, p := range n.peers {
		if p == nil {
			continue
		}
		ps := PeerSnapshot{
			NetworkID: p.id,
			Name:      p.name,
			UserID:    p.userID,
		}
		if p.addr != "" {
			ps.Addr = string(p.addr)
		}
		out = append(out, ps)
	}
	return out
}
