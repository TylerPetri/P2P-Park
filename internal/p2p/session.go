package p2p

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"p2p-park/internal/crypto/noiseconn"
	"p2p-park/internal/netx"
	"p2p-park/internal/proto"
	"time"
)

type deadlineConn interface {
	SetReadDeadline(t time.Time) error
}

func (n *Node) establishPeer(rawConn netx.Conn, inbound bool) (*peer, io.Closer, error) {
	id := n.Identity()

	ip := proto.NoiseIdentityPayload{
		Name:    n.cfg.Name,
		UserPub: id.SignPub,
	}
	payloadBytes, err := json.Marshal(ip)
	if err != nil {
		return nil, nil, err
	}

	var hs *noiseconn.HandshakeResult
	if inbound {
		hs, err = noiseconn.NewSecureServer(rawConn, id.NoisePriv[:], id.NoisePub[:], payloadBytes)
	} else {
		hs, err = noiseconn.NewSecureClient(rawConn, id.NoisePriv[:], id.NoisePub[:], payloadBytes)
	}
	if err != nil {
		return nil, nil, err
	}

	secure := hs.Conn

	// decode remote payload
	var remoteName string
	var remoteUserPub ed25519.PublicKey
	var remoteUserID string

	if len(hs.RemotePayload) > 0 {
		var rip proto.NoiseIdentityPayload
		if err := json.Unmarshal(hs.RemotePayload, &rip); err != nil {
			_ = secure.Close()
			return nil, nil, err
		}
		remoteName = rip.Name
		remoteUserPub = ed25519.PublicKey(rip.UserPub)
		remoteUserID = hex.EncodeToString(remoteUserPub)
	}

	dec := json.NewDecoder(bufio.NewReader(secure))
	enc := json.NewEncoder(secure)

	// hello handshake
	if err := n.sendHello(enc); err != nil {
		_ = secure.Close()
		return nil, nil, err
	}

	env, err := n.readEnvelopeWithTimeout(rawConn, dec, 5*time.Second)
	if err != nil {
		_ = secure.Close()
		return nil, nil, err
	}
	if env.Type != proto.MsgHello {
		_ = secure.Close()
		return nil, nil, errors.New("expected hello")
	}

	var hello proto.Hello
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		_ = secure.Close()
		return nil, nil, err
	}

	peerID := env.FromID
	pctx, cancel := context.WithCancel(n.ctx)
	p := &peer{
		id:           peerID,
		name:         remoteName,
		addr:         netx.Addr(hello.Listen),
		observedAddr: rawConn.RemoteAddr(),
		conn:         secure,
		writer:       enc,
		sendCh:       make(chan proto.Envelope, 128), // TODO: make configurable
		ctx:          pctx,
		cancel:       cancel,
		userPub:      remoteUserPub,
		userID:       remoteUserID,
	}

	if !n.addPeer(p) {
		_ = secure.Close()
		return nil, nil, nil
	}

	go p.writeLoop(n)
	return p, secure, nil
}

func (n *Node) runPeerReadLoop(p *peer) {
	dec := json.NewDecoder(bufio.NewReader(p.conn))

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

func (n *Node) readEnvelopeWithTimeout(rawConn netx.Conn, dec *json.Decoder, timeout time.Duration) (proto.Envelope, error) {
	if dc, ok := rawConn.(deadlineConn); ok {
		_ = dc.SetReadDeadline(time.Now().Add(timeout))
		defer func() { _ = dc.SetReadDeadline(time.Time{}) }()

		var env proto.Envelope
		err := dec.Decode(&env)
		return env, err
	}

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
		_ = rawConn.Close() // ensures decode unblocks and goroutine exits
		return proto.Envelope{}, errors.New("read timeout")
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
