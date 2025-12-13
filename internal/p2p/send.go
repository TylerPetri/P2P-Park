package p2p

import (
	"fmt"
	"p2p-park/internal/proto"
)

func (n *Node) sendPeerList(p *peer) error {
	pl := proto.PeerList{Peers: n.snapshotPeersInfo()}
	env := proto.Envelope{
		Type:    proto.MsgPeerList,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(pl),
	}
	n.sendAsync(p, env)
	return nil
}

func (n *Node) sendAsync(p *peer, env proto.Envelope) {
	select {
	case p.sendCh <- env:
		// queued
	default:
		n.Logf("peer %s send buffer full, dropping", p.id)
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
