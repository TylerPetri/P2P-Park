package sim

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"p2p-park/internal/dht"
	"p2p-park/internal/proto"
)

// Network is an in-process deterministic "transport" for DHT testing.
// It is NOT production networking; it exists to measure algorithmic behavior.
type Network struct {
	mu    sync.RWMutex
	nodes map[string]*Node // keyed by PeerID
	// Simulation knobs
	Latency   time.Duration // fixed latency per message
	DropRate  float64       // 0..1
	rng       *rand.Rand
}

func NewNetwork(seed int64) *Network {
	return &Network{
		nodes: make(map[string]*Node),
		rng:   rand.New(rand.NewSource(seed)),
	}
}

func (nw *Network) Add(node *Node) {
	nw.mu.Lock()
	nw.nodes[node.peerID] = node
	nw.mu.Unlock()
}

func (nw *Network) deliver(from *Node, toPeerID string, env proto.Envelope) error {
	nw.mu.RLock()
	to := nw.nodes[toPeerID]
	nw.mu.RUnlock()
	if to == nil {
		return fmt.Errorf("sim: unknown peer %s", toPeerID)
	}
	if nw.DropRate > 0 && nw.rng.Float64() < nw.DropRate {
		return nil
	}
	if nw.Latency > 0 {
		time.Sleep(nw.Latency)
	}
	to.dht.HandleDHT(to, from.peerID, from.addr, from.name, env)
	return nil
}

// Node implements dht.Sender for simulation.
type Node struct {
	nw     *Network
	peerID string
	addr   string
	name   string
	dht    *dht.DHT
}

func NewNode(nw *Network, peerID, addr, name string, d *dht.DHT) *Node {
	n := &Node{nw: nw, peerID: peerID, addr: addr, name: name, dht: d}
	nw.Add(n)
	return n
}

func (n *Node) ID() string { return n.peerID }

func (n *Node) SendToPeer(id string, env proto.Envelope) error {
	return n.nw.deliver(n, id, env)
}

func (n *Node) Logf(format string, args ...any) {}


func (n *Node) DHT() *dht.DHT { return n.dht }
