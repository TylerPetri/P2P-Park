package p2p

import "p2p-park/internal/proto"

func (n *Node) addPeer(p *peer) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.peers[p.id]; exists || p.id == n.id.ID {
		return false
	}
	if n.dht != nil {
		n.dht.OnPeerSeen(p.id, string(p.addr), p.name)
	}
	n.peers[p.id] = p
	n.emit(Event{Type: EventPeerConnected, PeerID: p.id, PeerAddr: string(p.addr), PeerName: p.name})
	return true
}

func (n *Node) removePeer(id string) {
	var p *peer

	n.mu.Lock()
	p = n.peers[id]
	if p != nil {
		delete(n.peers, id)

		if p.userID != "" {
			if cur := n.natByUserID[p.userID]; cur == p {
				delete(n.natByUserID, p.userID)
			}
		}
	}
	n.mu.Unlock()

	if p == nil {
		return
	}

	// Make removal idempotent
	p.once.Do(func() {
		if p.cancel != nil {
			p.cancel()
		}

		_ = p.conn.Close()

		n.emit(Event{Type: EventPeerDisconnected, PeerID: p.id, PeerAddr: string(p.addr), PeerName: p.name})
	})
}

func (n *Node) snapshotPeersInfo() []proto.PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]proto.PeerInfo, 0, len(n.peers))
	for _, p := range n.peers {
		if p == nil {
			continue
		}

		info := proto.PeerInfo{
			ID:   p.id,
			Name: p.name,
			Addr: string(p.addr),
		}

		if p.observedAddr != "" && n.cfg.IsSeed {
			info.PublicAddr = string(p.observedAddr)
		}

		out = append(out, info)
	}
	return out
}

func (n *Node) hasPeer(id string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	_, ok := n.peers[id]
	return ok
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

// PeerDisplayName returns the id (name) of the peer
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
