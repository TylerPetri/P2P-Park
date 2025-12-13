package p2p

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"p2p-park/internal/crypto/noiseconn"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
	"time"
)

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
