package p2p

import (
	"errors"
	"net"
	"strings"
	"time"
)

func (n *Node) acceptLoop() {
	for {
		conn, err := n.cfg.Network.Accept()
		if err != nil {

			select {
			case <-n.ctx.Done():
				return
			default:
			}

			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			n.Logf("accept error: %v", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		go n.handleConn(conn, true)
	}
}
