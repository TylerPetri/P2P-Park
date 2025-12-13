package p2p

import (
	"encoding/hex"
	"encoding/json"
	"p2p-park/internal/proto"
)

func (n *Node) sendNatRegister(p *peer) error {
	id := n.Identity()

	userID := hex.EncodeToString(id.SignPub)
	reg := proto.NatRegister{
		UserID: userID,
		Name:   n.cfg.Name,
	}

	env := proto.Envelope{
		Type:    proto.MsgNatRegister,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(reg),
	}
	n.sendAsync(p, env)
	return nil
}

func (n *Node) handleNatRegister(p *peer, env proto.Envelope) {
	if !n.cfg.IsSeed {
		return
	}

	var reg proto.NatRegister
	if err := json.Unmarshal(env.Payload, &reg); err != nil {
		n.Logf("bad NatRegister from %s: %v", p.id, err)
		return
	}
	if reg.UserID == "" {
		n.Logf("NatRegister from %s missing user_id", p.id)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.natByUserID == nil {
		n.natByUserID = make(map[string]*peer)
	}
	n.natByUserID[reg.UserID] = p

	p.userID = reg.UserID
	if p.name == "" && reg.Name != "" {
		p.name = reg.Name
	}

	n.Logf("NAT register: %s → peer %s", reg.UserID, p.id)
}

func (n *Node) handleNatRelaySeed(fromPeer *peer, env proto.Envelope) {
	if !n.cfg.IsSeed {
		return
	}

	var msg proto.NatRelay
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		n.Logf("bad NatRelay from %s: %v", fromPeer.id, err)
		return
	}
	if msg.ToUserID == "" {
		n.Logf("NatRelay from %s missing ToUserID", fromPeer.id)
		return
	}

	n.mu.RLock()
	target := n.natByUserID[msg.ToUserID]
	n.mu.RUnlock()

	if target == nil {
		n.Logf("NatRelay: no target for user %s", msg.ToUserID)
		return
	}

	// We re-wrap it in an Envelope so the target knows it came via NAT.
	fwdEnv := proto.Envelope{
		Type:    proto.MsgNatRelay,
		FromID:  fromPeer.id, // network ID of the original sender
		Payload: env.Payload, // same NatRelay payload
	}

	n.sendAsync(target, fwdEnv)
}

func (n *Node) handleNatRelayClient(fromPeer *peer, env proto.Envelope) {
	var msg proto.NatRelay
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		n.Logf("bad NatRelay inbound: %v", err)
		return
	}
	n.Logf("NatRelay from %s to %s; payload=%s", env.FromID, msg.ToUserID, string(msg.Payload))
}
