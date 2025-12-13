package p2p

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"log"
	"p2p-park/internal/crypto/noiseconn"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
	"p2p-park/internal/telemetry"
	"sync"
	"time"
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
	defer rawConn.Close()

	id := n.Identity()

	ip := proto.NoiseIdentityPayload{
		Name:    n.cfg.Name,
		UserPub: id.SignPub,
	}
	payloadBytes, err := json.Marshal(ip)
	if err != nil {
		n.Logf("marshal identity payload failed: %v", err)
		return
	}

	var hs *noiseconn.HandshakeResult
	if inbound {
		hs, err = noiseconn.NewSecureServer(rawConn, id.NoisePriv[:], id.NoisePub[:], payloadBytes)
	} else {
		hs, err = noiseconn.NewSecureClient(rawConn, id.NoisePriv[:], id.NoisePub[:], payloadBytes)
	}
	if err != nil {
		n.Logf("noise handshake failed (inboud=%v): %v", inbound, err)
		return
	}
	defer hs.Conn.Close()

	var remoteName string
	var remoteUserPub ed25519.PublicKey
	var remoteUserID string

	if len(hs.RemotePayload) > 0 {
		var rip proto.NoiseIdentityPayload
		if err := json.Unmarshal(hs.RemotePayload, &rip); err != nil {
			n.Logf("bad remote identity payload: %v", err)
			return
		}
		remoteName = rip.Name
		remoteUserPub = ed25519.PublicKey(rip.UserPub)
		remoteUserID = hex.EncodeToString(remoteUserPub)
	}

	r := bufio.NewReader(hs.Conn)
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(hs.Conn)

	// 1) Perform hello handshake over encrypted channel.
	if err := n.sendHello(enc); err != nil {
		n.Logf("send hello failed: %v", err)
		return
	}
	env, err := n.readEnvelope(dec, 5*time.Second)
	if err != nil {
		n.Logf("read hello failed: %v", err)
		return
	}
	if env.Type != proto.MsgHello {
		n.Logf("expected hello, got %s", env.Type)
		return
	}
	var hello proto.Hello
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		n.Logf("bad hello payload: %v", err)
		return
	}

	ra := rawConn.RemoteAddr()

	peerID := env.FromID
	p := &peer{
		id:           peerID,
		name:         remoteName,
		addr:         netx.Addr(hello.Listen),
		observedAddr: ra,
		conn:         rawConn,
		writer:       enc,
		sendCh:       make(chan proto.Envelope, 128), // TODO: revisit buffer size
		userPub:      remoteUserPub,
		userID:       remoteUserID,
	}

	if !n.addPeer(p) {
		n.Logf("duplicate peer %s; closing", p.id)
		return
	}
	go p.writeLoop(n.ctx, n)
	defer n.removePeer(p.id)

	if !n.cfg.IsSeed {
		if err := n.sendNatRegister(p); err != nil {
			n.Logf("send NAT register to %s failed: %v", p.id, err)
		}
	}

	if err := n.sendIdentify(p); err != nil {
		n.Logf("send identify to %s failed: %v", peerID, err)
	}

	n.Logf("connected to peer id=%s name=%s addr=%s inbound=%v", p.id, p.name, p.addr, inbound)

	// 2) Send our peer list to help them discover others.
	if err := n.sendPeerList(p); err != nil {
		n.Logf("send peer list to %s failed: %v", p.id, err)
	}

	// 3) Main read loop.
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		var env proto.Envelope
		if err := dec.Decode(&env); err != nil {
			n.Logf("read from %s failed: %v", p.id, err)
			return
		}
		n.handleEnvelope(p, env)
	}
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
	return enc.Encode(env)
}

func (n *Node) readEnvelope(dec *json.Decoder, timeout time.Duration) (proto.Envelope, error) {
	type result struct {
		env proto.Envelope
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var env proto.Envelope
		err := dec.Decode(&env)
		ch <- result{env: env, err: err}
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

	p, ok := n.peers[id]
	if !ok {
		return
	}
	delete(n.peers, id)

	close(p.sendCh)
	_ = p.conn.Close()

	n.Logf("peer %s removed", id)
}

func (n *Node) snapshotPeers() []proto.PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]proto.PeerInfo, 0, len(n.peers))
	for _, p := range n.peers {
		if p == nil {
			continue
		}

		info := proto.PeerInfo{
			ID:   p.id,
			Name: p.name,
			Addr: string(p.addr),
		}

		if p.observedAddr != "" && n.cfg.IsSeed {
			info.PublicAddr = string(p.observedAddr)
		}

		out = append(out, info)
	}
	return out
}

func (n *Node) handleEnvelope(p *peer, env proto.Envelope) {
	switch env.Type {
	case proto.MsgPeerList:
		var pl proto.PeerList
		if err := json.Unmarshal(env.Payload, &pl); err != nil {
			n.Logf("bad peer list from %s: %s", p.id, err)
			return
		}
		for _, pi := range pl.Peers {
			if pi.ID == n.id.ID {
				continue
			}
			if n.hasPeer(pi.ID) {
				continue
			}
			n.Logf("discovery: dialing peer %s at %s", pi.ID, pi.Addr)
			n.ConnectTo(netx.Addr(pi.Addr))
		}
	case proto.MsgGossip:
		select {
		case n.incoming <- env:
		default:
		}
		n.relay(p.id, env)
	case proto.MsgIdentify:
		n.handleIdentify(p, env)
	case proto.MsgNatRegister:
		n.handleNatRegister(p, env)
	case proto.MsgNatRelay:
		if n.cfg.IsSeed {
			n.handleNatRelaySeed(p, env)
		} else {
			n.handleNatRelayClient(p, env)
		}
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

func (n *Node) sendPeerList(p *peer) error {
	pl := proto.PeerList{Peers: n.snapshotPeers()}
	env := proto.Envelope{
		Type:    proto.MsgPeerList,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(pl),
	}
	n.sendAsync(p, env)
	return nil
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

func (n *Node) Logf(format string, args ...any) {
	if !n.cfg.Debug {
		return
	}
	if n.cfg.Logger != nil {
		n.cfg.Logger.Printf("[node %s] "+format, append([]any{n.id.ID[:8]}, args...)...)
	}
}

func (n *Node) sendNatRegister(p *peer) error {
	id := n.Identity()

	userID := hex.EncodeToString(id.SignPub)
	reg := proto.NatRegister{
		UserID: userID,
		Name:   n.cfg.Name,
	}

	env := proto.Envelope{
		Type:    proto.MsgNatRegister,
		FromID:  n.id.ID,
		Payload: proto.MustMarshal(reg),
	}
	n.sendAsync(p, env)
	return nil
}

func (n *Node) handleNatRegister(p *peer, env proto.Envelope) {
	if !n.cfg.IsSeed {
		return
	}

	var reg proto.NatRegister
	if err := json.Unmarshal(env.Payload, &reg); err != nil {
		n.Logf("bad NatRegister from %s: %v", p.id, err)
		return
	}
	if reg.UserID == "" {
		n.Logf("NatRegister from %s missing user_id", p.id)
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.natByUserID == nil {
		n.natByUserID = make(map[string]*peer)
	}
	n.natByUserID[reg.UserID] = p

	p.userID = reg.UserID
	if p.name == "" && reg.Name != "" {
		p.name = reg.Name
	}

	n.Logf("NAT register: %s → peer %s", reg.UserID, p.id)
}

func (n *Node) handleNatRelaySeed(fromPeer *peer, env proto.Envelope) {
	if !n.cfg.IsSeed {
		return
	}

	var msg proto.NatRelay
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		n.Logf("bad NatRelay from %s: %v", fromPeer.id, err)
		return
	}
	if msg.ToUserID == "" {
		n.Logf("NatRelay from %s missing ToUserID", fromPeer.id)
		return
	}

	n.mu.RLock()
	target := n.natByUserID[msg.ToUserID]
	n.mu.RUnlock()

	if target == nil {
		n.Logf("NatRelay: no target for user %s", msg.ToUserID)
		return
	}

	// We re-wrap it in an Envelope so the target knows it came via NAT.
	fwdEnv := proto.Envelope{
		Type:    proto.MsgNatRelay,
		FromID:  fromPeer.id, // network ID of the original sender
		Payload: env.Payload, // same NatRelay payload
	}

	n.sendAsync(target, fwdEnv)
}

func (n *Node) handleNatRelayClient(fromPeer *peer, env proto.Envelope) {
	var msg proto.NatRelay
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		n.Logf("bad NatRelay inbound: %v", err)
		return
	}
	n.Logf("NatRelay from %s to %s; payload=%s", env.FromID, msg.ToUserID, string(msg.Payload))
}
