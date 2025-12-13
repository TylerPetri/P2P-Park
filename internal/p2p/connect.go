package p2p

import (
	"p2p-park/internal/netx"
)

// ConnectTo allows manual dialing (used by discovery/bootstraps).
func (n *Node) ConnectTo(addr netx.Addr) error {
	conn, err := n.cfg.Network.Dial(addr)
	if err != nil {
		n.Logf("dial %s failed: %v", addr, err)
		return err
	}
	go n.handleConn(conn, false)
	return nil
}

func (n *Node) handleConn(rawConn netx.Conn, inbound bool) {
	p, secureCloser, err := n.establishPeer(rawConn, inbound)
	if err != nil {
		n.Logf("conn setup failed (inbound=%v): %v", inbound, err)
		_ = rawConn.Close()
		return
	}
	if p == nil {
		_ = rawConn.Close()
		return
	}
	defer func() {
		n.removePeer(p.id)
		if secureCloser != nil {
			_ = secureCloser.Close()
		}
	}()

	if !n.cfg.IsSeed {
		if err := n.sendNatRegister(p); err != nil {
			n.Logf("send NAT register to %s failed: %v", p.id, err)
		}
	}

	if err := n.sendIdentify(p); err != nil {
		n.Logf("send identify to %s failed: %v", p.id, err)
	}

	n.Logf("connected to peer id=%s name=%s addr=%s inbound=%v", p.id, p.name, p.addr, inbound)

	if err := n.sendPeerList(p); err != nil {
		n.Logf("send peer list to %s failed: %v", p.id, err)
	}

	n.runPeerReadLoop(p)
}
