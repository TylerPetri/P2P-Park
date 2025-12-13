package p2p

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"log"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
	"p2p-park/internal/telemetry"
	"sync"
)

type NodeConfig struct {
	Name       string           // user-facing name
	Network    netx.Network     // transport implementation
	BindAddr   string           // e.g. ":0" to choose random port
	Bootstraps []netx.Addr      // known peers to try on startup
	Protocol   string           // protocol version string
	Logger     telemetry.Logger // system logger
	Debug      bool             // flag for showing hidden logs to debug
	IsSeed     bool             // if true, this node will keep NAT registry & relay
}

type peer struct {
	id           string
	addr         netx.Addr
	observedAddr netx.Addr
	conn         netx.Conn
	writer       *json.Encoder

	sendCh chan proto.Envelope

	name    string
	userPub ed25519.PublicKey
	userID  string
}

// PeerSnapshot is a read-only view of a connected peer.
type PeerSnapshot struct {
	NetworkID string // Noise hex ID (p.id)
	Name      string // p.name from Identify
	UserID    string // hex(ed25519 pub) if known
	Addr      string // listen address string
}

type Node struct {
	cfg  NodeConfig
	id   *Identity
	addr netx.Addr

	mu    sync.RWMutex
	peers map[string]*peer

	ctx    context.Context
	cancel context.CancelFunc

	incoming    chan proto.Envelope // gossip and other messages
	natByUserID map[string]*peer    // only meaningful when cfg.IsSeed == true

	events chan Event
}

func NewNode(cfg NodeConfig) (*Node, error) {
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	id, err := NewIdentity()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		cfg:      cfg,
		id:       id,
		peers:    make(map[string]*peer),
		ctx:      ctx,
		cancel:   cancel,
		incoming: make(chan proto.Envelope, 128),
		events:   make(chan Event, 128),
	}
	if cfg.IsSeed {
		n.natByUserID = make(map[string]*peer)
	}
	return n, nil
}

// ID returns this node's peer ID.
func (n *Node) ID() string { return n.id.ID }

// Identity returns the node's identity (public/private keypair).
func (n *Node) Identity() *Identity { return n.id }

// ListenAddr returns where this node is listening.
func (n *Node) ListenAddr() netx.Addr { return n.addr }

// Incoming returns a channel of messages for higher-level app logic.
func (n *Node) Incoming() <-chan proto.Envelope { return n.incoming }

// Name returns this node's name
func (n *Node) Name() string { return n.cfg.Name }

// Events return a channel of events for logging
func (n *Node) Events() <-chan Event { return n.events }

// Start brings the node online.
func (n *Node) Start() error {
	addr, err := n.cfg.Network.Listen(n.cfg.BindAddr)
	if err != nil {
		return err
	}
	n.addr = addr
	n.Logf("listening on %s, peerID=%s", n.addr, n.id.ID)

	go n.acceptLoop()

	go n.discoveryLoop()

	return nil
}

// Stop shuts down the node.
func (n *Node) Stop() error {
	n.cancel()
	return n.cfg.Network.Close()
}

func (n *Node) emit(e Event) {
	select {
	case n.events <- e:
	default:
		// drop to avoid deadlock; optionally log once
	}
}

// Broadcast sends a gossip mesage to all connected peers.
func (n *Node) Broadcast(g proto.Gossip) {
	env := proto.Envelope{
		Type:    proto.MsgGossip,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(g),
	}
	n.relay(n.id.ID, env)
}

func (n *Node) relay(originID string, env proto.Envelope) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for id, p := range n.peers {
		if id == originID {
			continue
		}
		n.sendAsync(p, env)
	}
}
