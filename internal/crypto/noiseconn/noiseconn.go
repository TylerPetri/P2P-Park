package noiseconn

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// SecureConn wraps an underlying stream with Noise cipher states.
type SecureConn struct {
	underlying io.ReadWriteCloser

	readCS  *noise.CipherState
	writeCS *noise.CipherState
}

// Read reads a single length-prefixed encrypted frame and decrypts it.
func (c *SecureConn) Read(p []byte) (int, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.underlying, lenBuf[:]); err != nil {
		return 0, err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n == 0 {
		return 0, fmt.Errorf("invalid frame length")
	}

	ct := make([]byte, n)
	if _, err := io.ReadFull(c.underlying, ct); err != nil {
		return 0, err
	}

	pt, err := c.readCS.Decrypt(nil, nil, ct)
	if err != nil {
		return 0, err
	}

	if len(pt) > len(p) {
		copy(p, pt[:len(p)])
		return len(p), io.ErrShortBuffer
	}
	copy(p, pt)
	return len(pt), nil
}

// Write encrypts p as a single frame and writes it with a length prefix.
func (c *SecureConn) Write(p []byte) (int, error) {
	ct, err := c.writeCS.Encrypt(nil, nil, p)
	if err != nil {
		return 0, err
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(ct)))

	if _, err := c.underlying.Write(lenBuf[:]); err != nil {
		return 0, err
	}
	if _, err := c.underlying.Write(ct); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *SecureConn) Close() error {
	return c.underlying.Close()
}

// HandshakeResult contains the secure connection plus the remote's identity payload.
type HandshakeResult struct {
	Conn          *SecureConn
	RemotePayload []byte
}

// NewSecureClient runs a Noise_XX handshake as initiator and attaches localPayload
// as the final handshake payload (identity, etc.).
func NewSecureClient(
	underlying io.ReadWriteCloser,
	staticPriv, staticPub, localPayload []byte,
) (*HandshakeResult, error) {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

	cfg := noise.Config{
		CipherSuite:   cs,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXX,
		Initiator:     true,
		StaticKeypair: noise.DHKey{Private: staticPriv, Public: staticPub},
	}

	hs, err := noise.NewHandshakeState(cfg)
	if err != nil {
		return nil, err
	}

	// XX pattern: 3 messages total.

	// -> e
	msg1, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, err
	}
	if err := writeHandshakeMsg(underlying, msg1); err != nil {
		return nil, err
	}

	// <- e, ee, s, es
	msg2, err := readHandshakeMsg(underlying)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := hs.ReadMessage(nil, msg2); err != nil {
		return nil, err
	}

	// -> s, se, payload (our identity payload)
	msg3, cs1, cs2, err := hs.WriteMessage(nil, localPayload)
	if err != nil {
		return nil, err
	}
	if err := writeHandshakeMsg(underlying, msg3); err != nil {
		return nil, err
	}

	// Per noise docs, for the party that *writes* the final message:
	//   first CipherState = sending, second = receiving.
	return &HandshakeResult{
		Conn: &SecureConn{
			underlying: underlying,
			readCS:     cs2, // receiving
			writeCS:    cs1, // sending
		},
		RemotePayload: nil, // XX here carries payload only from initiator -> responder
	}, nil
}

// NewSecureServer runs a Noise_XX handshake as responder and returns a SecureConn,
// plus the remote's identity payload (from initiator).
func NewSecureServer(
	underlying io.ReadWriteCloser,
	staticPriv, staticPub, _ []byte, // localPayload currently unused
) (*HandshakeResult, error) {
	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

	cfg := noise.Config{
		CipherSuite:   cs,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeXX,
		Initiator:     false,
		StaticKeypair: noise.DHKey{Private: staticPriv, Public: staticPub},
	}

	hs, err := noise.NewHandshakeState(cfg)
	if err != nil {
		return nil, err
	}

	// <- e
	msg1, err := readHandshakeMsg(underlying)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := hs.ReadMessage(nil, msg1); err != nil {
		return nil, err
	}

	// -> e, ee, s, es
	msg2, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, err
	}
	if err := writeHandshakeMsg(underlying, msg2); err != nil {
		return nil, err
	}

	// <- s, se, payload (initiator's identity payload)
	msg3, err := readHandshakeMsg(underlying)
	if err != nil {
		return nil, err
	}
	remotePayload, cs1, cs2, err := hs.ReadMessage(nil, msg3)
	if err != nil {
		return nil, err
	}

	// For the party that *reads* the final message:
	//   first CipherState = receiving, second = sending.
	return &HandshakeResult{
		Conn: &SecureConn{
			underlying: underlying,
			readCS:     cs1, // receiving
			writeCS:    cs2, // sending
		},
		RemotePayload: remotePayload,
	}, nil
}
