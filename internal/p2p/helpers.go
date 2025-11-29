package p2p

import "p2p-park/internal/proto"

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
	return p.writer.Encode(env)
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
