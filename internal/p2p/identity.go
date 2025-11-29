package p2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/curve25519"
)

type Identity struct {
	SignPriv ed25519.PrivateKey
	SignPub  ed25519.PublicKey

	NoisePriv [32]byte
	NoisePub  [32]byte

	ID string // hex-encoded public key
}

// PlayerIDFromPub derives the canonical player ID from a public key.
func PlayerIDFromPub(pub ed25519.PublicKey) string {
	return hex.EncodeToString(pub)
}

// noiseKeypair generates an X25519 keypair.
func noiseKeypair() (priv, pub [32]byte, err error) {
	if _, err = io.ReadFull(rand.Reader, priv[:]); err != nil {
		return
	}
	// Clamp as per X25519 spec.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	curve25519.ScalarBaseMult(&pub, &priv)
	return
}

func NewIdentity() (*Identity, error) {
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	nPriv, nPub, err := noiseKeypair()

	id := hex.EncodeToString(nPub[:])

	return &Identity{
		SignPriv:  signPriv,
		SignPub:   signPub,
		NoisePriv: nPriv,
		NoisePub:  nPub,
		ID:        id,
	}, nil
}
