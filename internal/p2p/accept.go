package p2p

func (n *Node) acceptLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		conn, err := n.cfg.Network.Accept()
		if err != nil {
			n.Logf("accept error: %v", err)
			return
		}
		go n.handleConn(conn, true)
	}
}
