package p2p

import "context"

func (p *peer) writeLoop(ctx context.Context, n *Node) {
	for {
		select {
		case env, ok := <-p.sendCh:
			if !ok {
				return
			}
			if err := p.writer.Encode(env); err != nil {
				n.Logf("write to %s failed: %v", p.id, err)
				go n.removePeer(p.id)
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
