package dht

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"p2p-park/internal/proto"
)

var ErrInvalidRecord = errors.New("dht: invalid record")

func (d *DHT) ValidateRecordAgainstKey(key [32]byte, rec *proto.DHTRecord) error {
	if rec == nil {
		return ErrInvalidRecord
	}

	// Expiry sanity
	if rec.ExpiresUnix != 0 && time.Now().Unix() > rec.ExpiresUnix {
		return ErrInvalidRecord
	}

	// Size caps (adjust later)
	const maxValue = 64 * 1024
	if len(rec.Value) > maxValue {
		return ErrRecordTooLarge
	}

	switch rec.Type {
	case RecordImmutable:
		sum := sha256.Sum256(rec.Value)
		if sum != key {
			return ErrKeyMismatch
		}
		return nil

	case RecordMutable:
		if len(rec.PubKey) != ed25519.PublicKeySize || len(rec.Sig) == 0 {
			return ErrBadRecord
		}
		pub := ed25519.PublicKey(rec.PubKey)
		// key must equal sha256(pub || name)
		expect := KeyFromMutable(pub, rec.Name)
		if expect != key {
			return ErrKeyMismatch
		}
		if !VerifyMutable(pub, key, rec.Seq, rec.ExpiresUnix, rec.Value, rec.Sig) {
			return ErrBadSignature
		}
		return nil

	default:
		return ErrBadRecord
	}
}
