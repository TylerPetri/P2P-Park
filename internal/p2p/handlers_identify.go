package p2p

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"p2p-park/internal/proto"
)

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

func (n *Node) handleIdentify(p *peer, env proto.Envelope) {
	var ident proto.Identify
	if err := json.Unmarshal(env.Payload, &ident); err != nil {
		n.Logf("bad identify from %s: %v", p.id, err)
		return
	}

	n.mu.Lock()
	p.name = ident.Name
	if len(ident.UserPub) == ed25519.PublicKeySize {
		p.userPub = ed25519.PublicKey(ident.UserPub)
		p.userID = hex.EncodeToString(ident.UserPub)
	}
	n.mu.Unlock()

	if len(ident.UserPub) != ed25519.PublicKeySize {
		n.Logf("identify from %s has invalid user_pub length %d", p.id, len(ident.UserPub))
	}

	n.Logf("peer %s identified as %q (userID=%s)", p.id, p.name, p.userID)
}
