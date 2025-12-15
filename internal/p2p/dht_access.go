package p2p

import "p2p-park/internal/dht"

// dhtAccessor returns the node's DHT engine.
// It is intentionally unexported; tests in package p2p may use it.
func (n *Node) dhtAccessor() *dht.DHT {
	return n.dht
}
