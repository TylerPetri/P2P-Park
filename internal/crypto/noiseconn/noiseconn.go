package noiseconn

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// SecureConn wraps an underlying stream with Noise cipher states
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

// Write encrypts p as a single frame and write it with a length prefix.
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

// NewSecureClient runs a Noise_XX handshake as initiator and returns a SecureConn.
func NewSecureClient(underlying io.ReadWriteCloser, staticPriv, staticPub []byte) (*SecureConn, error) {
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

	// -> e
	msg, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, err
	}
	if _, err := underlying.Write(msg); err != nil {
		return nil, err
	}

	// <- e, ee, s, es
	buf := make([]byte, 65535)
	n, err := underlying.Read(buf)
	if err != nil {
		return nil, err
	}
	_, _, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, err
	}

	// -> s, se
	msg2, cs1, cs2, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, err
	}
	if _, err := underlying.Write(msg2); err != nil {
		return nil, err
	}

	// Note: convention is cs2 for read and cs1 for write, the noise docs specify the order; if messages decypt wrong, swap.
	return &SecureConn{
		underlying: underlying,
		readCS:     cs2,
		writeCS:    cs1,
	}, nil
}

// NewSecureServer runs a Noise_XX handshake as responder and returns a SecureConn.
func NewSecureServer(underlying io.ReadWriteCloser, staticPriv, staticPub []byte) (*SecureConn, error) {
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
	buf := make([]byte, 65535)
	n, err := underlying.Read(buf)
	if err != nil {
		return nil, err
	}
	_, _, _, err = hs.ReadMessage(nil, buf[:n])
	if err != nil {
		return nil, err
	}

	// -> e, ee, s, es
	msg, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, err
	}
	if _, err := underlying.Write(msg); err != nil {
		return nil, err
	}

	// <- s, se
	n2, err := underlying.Read(buf)
	if err != nil {
		return nil, err
	}
	_, cs1, cs2, err := hs.ReadMessage(nil, buf[:n2])
	if err != nil {
		return nil, err
	}

	// For responder, cipher state order is swapped relative to initiator.
	return &SecureConn{
		underlying: underlying,
		readCS:     cs1,
		writeCS:    cs2,
	}, nil
}
