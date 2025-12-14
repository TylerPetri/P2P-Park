package p2p

func (p *peer) writeLoop(n *Node) {
	for {
		select {
		case <-p.ctx.Done():
			return

		case env, ok := <-p.sendCh:
			if !ok {
				return
			}
			if err := p.writer.Encode(env); err != nil {
				n.Logf("write to %s failed: %v", p.id, err)
				go n.removePeer(p.id)
				return
			}
		}
	}
}
