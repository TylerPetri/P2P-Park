package p2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
)

type Identity struct {
	Priv ed25519.PrivateKey
	Pub  ed25519.PublicKey
	ID   string // hex-encoded public key
}

func NewIdentity() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	id := hex.EncodeToString(pub)
	return &Identity{
		Priv: priv,
		Pub:  pub,
		ID:   id,
	}, nil
}

// TODO : add persistence (save/load keypair) so user keeps same ID across runs.
