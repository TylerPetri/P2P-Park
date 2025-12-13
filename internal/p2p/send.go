package p2p

import (
	"fmt"
	"p2p-park/internal/proto"
)

type SendPolicy int

const (
	SendDrop       SendPolicy = iota // drop this message if buffer full
	SendDisconnect                   // drop peer if buffer full
)

func (n *Node) sendAsyncWithPolicy(p *peer, env proto.Envelope, policy SendPolicy) {
	select {
	case <-p.ctx.Done():
		// peer is closing; just drop
		return
	default:
	}

	select {
	case p.sendCh <- env:
		return
	default:
		if policy == SendDisconnect {
			// Important: do not call removePeer synchronously here.
			go n.removePeer(p.id)
			n.Logf("dropping peer %s: send buffer full", p.id)
		}
		// else: drop the message
	}
}

func (n *Node) sendAsync(p *peer, env proto.Envelope) {
	n.sendAsyncWithPolicy(p, env, SendDisconnect)
}

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
