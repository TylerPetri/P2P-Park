package p2p

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"p2p-park/internal/proto"
)

func (n *Node) handleIdentify(p *peer, env proto.Envelope) {
	var ident proto.Identify
	if err := json.Unmarshal(env.Payload, &ident); err != nil {
		n.logf("bad identify from %s: %v", p.id, err)
		return
	}

	p.name = ident.Name
	if len(ident.UserPub) == ed25519.PublicKeySize {
		p.userPub = ed25519.PublicKey(ident.UserPub)
		p.userID = hex.EncodeToString(ident.UserPub)
	} else {
		n.logf("identify from %s has invalid user_pub length %d", p.id, len(ident.UserPub))
	}

	n.logf("peer %s identified as %q (userID=%s)", p.id, p.name, p.userID)
}
