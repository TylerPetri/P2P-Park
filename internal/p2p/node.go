package p2p

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log"
	"p2p-park/internal/crypto/noiseconn"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
	"sync"
	"time"
)

type NodeConfig struct {
	Name       string       // user-facing name
	Network    netx.Network // transport implementation
	BindAddr   string       // e.g. ":0" to choose random port
	Bootstraps []netx.Addr  // known peers to try on startup
	Protocol   string       // protocol version string
	Logger     *log.Logger
}

type peer struct {
	id     string
	name   string
	addr   netx.Addr
	conn   netx.Conn
	writer *json.Encoder
}

type Node struct {
	cfg  NodeConfig
	id   *Identity
	addr netx.Addr

	mu    sync.RWMutex
	peers map[string]*peer

	ctx    context.Context
	cancel context.CancelFunc

	incoming chan proto.Envelope // gossip and other messages
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
	return &Node{
		cfg:      cfg,
		id:       id,
		peers:    make(map[string]*peer),
		ctx:      ctx,
		cancel:   cancel,
		incoming: make(chan proto.Envelope, 128),
	}, nil
}

// ID returns this node's peer ID.
func (n *Node) ID() string { return n.id.ID }

// Identity returns the node's identity (public/private keypair).
func (n *Node) Identity() *Identity { return n.id }

// ListenAddr returns where this node is listening.
func (n *Node) ListenAddr() netx.Addr { return n.addr }

// Incoming returns a channel of messages for higher-level app logic.
func (n *Node) Incoming() <-chan proto.Envelope { return n.incoming }

// Start brings the node online.
func (n *Node) Start() error {
	addr, err := n.cfg.Network.Listen(n.cfg.BindAddr)
	if err != nil {
		return err
	}
	n.addr = addr
	n.logf("listening on %s, peerID=%s", n.addr, n.id.ID)

	go n.acceptLoop()

	go n.discoveryLoop()

	return nil
}

// Stop shuts down the node.
func (n *Node) Stop() error {
	n.cancel()
	return n.cfg.Network.Close()
}

func (n *Node) acceptLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		conn, err := n.cfg.Network.Accept()
		if err != nil {
			n.logf("accept error: %v", err)
			return
		}
		go n.handleConn(conn, true)
	}
}

// ConnectTo allows manual dialing (used by discovery/bootstraps).
func (n *Node) ConnectTo(addr netx.Addr) {
	go func() {
		conn, err := n.cfg.Network.Dial(addr)
		if err != nil {
			n.logf("dial %s failed: %v", addr, err)
			return
		}
		n.handleConn(conn, false)
	}()
}

func (n *Node) handleConn(rawConn netx.Conn, inbound bool) {
	defer rawConn.Close()

	id := n.Identity()

	var sc io.ReadWriteCloser
	var err error
	if inbound {
		sc, err = noiseconn.NewSecureServer(rawConn, id.NoisePriv[:], id.NoisePub[:])
	} else {
		sc, err = noiseconn.NewSecureClient(rawConn, id.NoisePriv[:], id.NoisePub[:])
	}
	if err != nil {
		n.logf("noise handshake failed (inboud=%v): %v", inbound, err)
		return
	}
	defer sc.Close()

	r := bufio.NewReader(sc)
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(sc)

	// 1) Perform hello handshake over encrypted channel.
	if err := n.sendHello(enc); err != nil {
		n.logf("send hello failed: %v", err)
		return
	}
	env, err := n.readEnvelope(dec, 5*time.Second)
	if err != nil {
		n.logf("read hello failed: %v", err)
		return
	}
	if env.Type != proto.MsgHello {
		n.logf("expected hello, got %s", env.Type)
		return
	}
	var hello proto.Hello
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		n.logf("bad hello payload: %v", err)
		return
	}

	peerID := env.FromID
	p := &peer{
		id:     peerID,
		name:   hello.Name,
		addr:   netx.Addr(hello.Listen),
		conn:   rawConn,
		writer: enc,
	}

	if !n.addPeer(p) {
		n.logf("duplicate peer %s; closing", p.id)
		return
	}
	defer n.removePeer(p.id)

	n.logf("connected to peer id=%s name=%s addr=%s inbound=%v", p.id, p.name, p.addr, inbound)

	// 2) Send our peer list to help them discover others.
	if err := n.sendPeerList(p); err != nil {
		n.logf("send peer list to %s failed: %v", p.id, err)
	}

	// 3) Main read loop.
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		var senv proto.SignedEnvelope
		if err := dec.Decode(&senv); err != nil {
			n.logf("read from %s failed: %v", p.id, err)
			return
		}
		env, ok := n.verifySignedEnvelope(senv)
		if !ok {
			n.logf("invalid signed envelope from %s; dropping", p.id)
			continue
		}
		n.handleEnvelope(p, env)
	}
}

// signEnvelope signs an Envelope using this node's private key.
func (n *Node) signEnvelope(env proto.Envelope) (proto.SignedEnvelope, error) {
	data, err := proto.EncodeEnvelopeCanonical(env)
	if err != nil {
		return proto.SignedEnvelope{}, err
	}
	sig := ed25519.Sign(n.id.SignPriv, data)
	return proto.SignedEnvelope{
		Envelope:  env,
		PubKey:    n.id.SignPub,
		Signature: sig,
	}, nil
}

// verifySignedEnvelope verifies signature AND that FromID matches pubkey.
// Returns the inner Envelope if valid.
func (n *Node) verifySignedEnvelope(senv proto.SignedEnvelope) (proto.Envelope, bool) {
	data, err := proto.EncodeEnvelopeCanonical(senv.Envelope)
	if err != nil {
		return proto.Envelope{}, false
	}
	if len(senv.PubKey) != ed25519.PublicKeySize {
		return proto.Envelope{}, false
	}
	pub := ed25519.PublicKey(senv.PubKey)

	if !ed25519.Verify(pub, data, senv.Signature) {
		return proto.Envelope{}, false
	}

	expectedID := PlayerIDFromPub(pub)
	if senv.Envelope.FromID != expectedID {
		return proto.Envelope{}, false
	}

	return senv.Envelope, true
}

func (n *Node) sendHello(enc *json.Encoder) error {
	h := proto.Hello{
		Name:     n.cfg.Name,
		Listen:   string(n.addr),
		Protocol: n.cfg.Protocol,
	}
	env := proto.Envelope{
		Type:    proto.MsgHello,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(h),
	}
	senv, err := n.signEnvelope(env)
	if err != nil {
		return err
	}
	return enc.Encode(senv)
}

func (n *Node) readEnvelope(dec *json.Decoder, timeout time.Duration) (proto.Envelope, error) {
	type result struct {
		env proto.Envelope
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var senv proto.SignedEnvelope
		err := dec.Decode(&senv)
		if err != nil {
			ch <- result{err: err}
			return
		}
		env, ok := n.verifySignedEnvelope(senv)
		if !ok {
			ch <- result{err: context.Canceled}
			return
		}
		ch <- result{env: env, err: nil}
	}()

	select {
	case r := <-ch:
		return r.env, r.err
	case <-time.After(timeout):
		return proto.Envelope{}, context.DeadlineExceeded
	}
}

func (n *Node) addPeer(p *peer) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, exists := n.peers[p.id]; exists || p.id == n.id.ID {
		return false
	}
	n.peers[p.id] = p
	return true
}

func (n *Node) removePeer(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.peers, id)
}

func (n *Node) snapshotPeers() []proto.PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]proto.PeerInfo, 0, len(n.peers))
	for _, p := range n.peers {
		out = append(out, proto.PeerInfo{
			ID:   p.id,
			Addr: string(p.addr),
			Name: p.name,
		})
	}
	return out
}

func (n *Node) sendPeerList(p *peer) error {
	pl := proto.PeerList{Peers: n.snapshotPeers()}
	env := proto.Envelope{
		Type:    proto.MsgPeerList,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(pl),
	}
	senv, err := n.signEnvelope(env)
	if err != nil {
		return err
	}
	return p.writer.Encode(senv)
}

func (n *Node) handleEnvelope(from *peer, env proto.Envelope) {
	switch env.Type {
	case proto.MsgPeerList:
		var pl proto.PeerList
		if err := json.Unmarshal(env.Payload, &pl); err != nil {
			n.logf("bad peer list from %s: %s", from.id, err)
			return
		}
		for _, pi := range pl.Peers {
			if pi.ID == n.id.ID {
				continue
			}
			if n.hasPeer(pi.ID) {
				continue
			}
			n.logf("discovery: dialing peer %s at %s", pi.ID, pi.Addr)
			n.ConnectTo(netx.Addr(pi.Addr))
		}
	case proto.MsgGossip:
		select {
		case n.incoming <- env:
		default:
		}
		n.relay(from.id, env)
	default:
		select {
		case n.incoming <- env:
		default:
		}
	}
}

func (n *Node) hasPeer(id string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	_, ok := n.peers[id]
	return ok
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
	senv, err := n.signEnvelope(env)
	if err != nil {
		n.logf("relay sign failed: %v", err)
		return
	}

	n.mu.RLock()
	defer n.mu.RUnlock()
	for id, p := range n.peers {
		if id == originID {
			continue
		}
		if err := p.writer.Encode(senv); err != nil {
			n.logf("relay to %s failed: %v", id, err)
		}
	}
}

func (n *Node) logf(format string, args ...any) {
	if n.cfg.Logger != nil {
		n.cfg.Logger.Printf("[node %s] "+format, append([]any{n.id.ID[:8]}, args...)...)
	}
}
