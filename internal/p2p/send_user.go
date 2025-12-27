package p2p

import (
	"fmt"

	"p2p-park/internal/proto"
)

// NetworkPeerIDForUserID returns the connected network peer id (Noise public key hex)
// for a given userID (hex(ed25519 pub)), if currently connected.
func (n *Node) NetworkPeerIDForUserID(userID string) (string, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	p := n.peersByUserID[userID]
	if p == nil {
		return "", false
	}
	return p.id, true
}

// SendToUserID sends an envelope to a connected peer addressed by userID (hex(ed25519 pub)).
func (n *Node) SendToUserID(userID string, env proto.Envelope) error {
	pid, ok := n.NetworkPeerIDForUserID(userID)
	if !ok {
		return fmt.Errorf("unknown user %q (not connected)", userID)
	}
	return n.SendToPeer(pid, env)
}
