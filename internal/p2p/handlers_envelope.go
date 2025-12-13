package p2p

import (
	"encoding/json"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
)

func (n *Node) handleEnvelope(p *peer, env proto.Envelope) {
	switch env.Type {
	case proto.MsgPeerList:
		var pl proto.PeerList
		if err := json.Unmarshal(env.Payload, &pl); err != nil {
			n.Logf("bad peer list from %s: %s", p.id, err)
			return
		}
		for _, pi := range pl.Peers {
			if pi.ID == n.id.ID {
				continue
			}
			if n.hasPeer(pi.ID) {
				continue
			}
			n.Logf("discovery: dialing peer %s at %s", pi.ID, pi.Addr)
			n.ConnectTo(netx.Addr(pi.Addr))
		}
	case proto.MsgGossip:
		select {
		case n.incoming <- env:
		default:
		}
		n.relay(p.id, env)
	case proto.MsgIdentify:
		n.handleIdentify(p, env)
	case proto.MsgNatRegister:
		n.handleNatRegister(p, env)
	case proto.MsgNatRelay:
		if n.cfg.IsSeed {
			n.handleNatRelaySeed(p, env)
		} else {
			n.handleNatRelayClient(p, env)
		}
	default:
		select {
		case n.incoming <- env:
		default:
		}
	}
}
