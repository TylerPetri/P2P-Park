package p2p

import (
	"io"
	"log"
	"testing"
	"time"

	"p2p-park/internal/netx"
)

type nodeTestOpt func(*NodeConfig)

// WithSeed makes the node a seed (if you want to test NAT registry/relay later).
func WithSeed(isSeed bool) nodeTestOpt {
	return func(cfg *NodeConfig) { cfg.IsSeed = isSeed }
}

// WithLogger lets you override the logger (default is io.Discard).
func WithLogger(l *log.Logger) nodeTestOpt {
	return func(cfg *NodeConfig) { cfg.Logger = l }
}

// WithProtocol overrides the protocol string (default "test/0").
func WithProtocol(p string) nodeTestOpt {
	return func(cfg *NodeConfig) { cfg.Protocol = p }
}

// WithDebug toggles debug mode.
func WithDebug(debug bool) nodeTestOpt {
	return func(cfg *NodeConfig) { cfg.Debug = debug }
}

// WithBootstraps sets the seed address
func WithBootstraps(addrs ...netx.Addr) nodeTestOpt {
	return func(cfg *NodeConfig) { cfg.Bootstraps = addrs }
}

// newTestNode spins up a node bound to an ephemeral localhost port and auto-stops it.
func newTestNode(t *testing.T, name string, opts ...nodeTestOpt) *Node {
	t.Helper()

	cfg := NodeConfig{
		Name:       name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   "127.0.0.1:0",
		Bootstraps: nil,
		Protocol:   "test/0",
		Logger:     log.New(io.Discard, "", log.LstdFlags),
		Debug:      true,
		IsSeed:     false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode(%s) error: %v", name, err)
	}
	if err := n.Start(); err != nil {
		t.Fatalf("Start(%s) error: %v", name, err)
	}

	t.Cleanup(func() { _ = n.Stop() })
	return n
}

func waitPeers(t *testing.T, n *Node, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if n.PeerCount() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for peers: node=%s have=%d want=%d", n.Name(), n.PeerCount(), want)
}

func connect(t *testing.T, from, to *Node) {
	t.Helper()
	if err := from.ConnectTo(to.ListenAddr()); err != nil {
		t.Fatalf("%s.ConnectTo(%s) error: %v", from.Name(), to.Name(), err)
	}
}

// connectTriangle connects b->a, c->b, a->c and waits for each to have 2 peers.
func connectTriangle(t *testing.T, a, b, c *Node) {
	t.Helper()
	connect(t, b, a)
	connect(t, c, b)
	connect(t, a, c)

	waitPeers(t, a, 2, 3*time.Second)
	waitPeers(t, b, 2, 3*time.Second)
	waitPeers(t, c, 2, 3*time.Second)
}

func drainIncomingForever(t *testing.T, n *Node, done <-chan struct{}) {
	t.Helper()
	go func() {
		for {
			select {
			case <-done:
				return
			case _, ok := <-n.Incoming():
				if !ok {
					return
				}
			}
		}
	}()
}
