package dht

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	RecordImmutable = "IMMUTABLE"
	RecordMutable   = "MUTABLE"
)

var (
	ErrBadRecord      = errors.New("dht: bad record")
	ErrBadSignature   = errors.New("dht: bad signature")
	ErrSeqTooLow      = errors.New("dht: seq too low")
	ErrKeyMismatch    = errors.New("dht: key mismatch")
	ErrRecordTooLarge = errors.New("dht: record too large")
)

func KeyFromImmutable(value []byte) [32]byte {
	return sha256.Sum256(value)
}

// For mutable keys: key = sha256(pubKey || name)
func KeyFromMutable(pubKey ed25519.PublicKey, name string) [32]byte {
	h := sha256.New()
	h.Write(pubKey)
	h.Write([]byte(name))
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func KeyHex(k [32]byte) string { return hex.EncodeToString(k[:]) }

func ParseKeyHex(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return out, ErrBadRecord
	}
	copy(out[:], b)
	return out, nil
}

// Canonical payload for signing mutable records.
func signPayload(key [32]byte, seq uint64, expiresUnix int64, value []byte) []byte {
	buf := make([]byte, 0, 32+8+8+len(value))
	buf = append(buf, key[:]...)

	// seq (big endian)
	for i := 7; i >= 0; i-- {
		buf = append(buf, byte(seq>>(8*i)))
	}

	// expiresUnix (big endian uint64)
	u := uint64(expiresUnix)
	for i := 7; i >= 0; i-- {
		buf = append(buf, byte(u>>(8*i)))
	}

	buf = append(buf, value...)
	sum := sha256.Sum256(buf)
	return sum[:]
}

func SignMutable(priv ed25519.PrivateKey, key [32]byte, seq uint64, expiresUnix int64, value []byte) []byte {
	msg := signPayload(key, seq, expiresUnix, value)
	return ed25519.Sign(priv, msg)
}

func VerifyMutable(pub ed25519.PublicKey, key [32]byte, seq uint64, expiresUnix int64, value []byte, sig []byte) bool {
	msg := signPayload(key, seq, expiresUnix, value)
	return ed25519.Verify(pub, msg, sig)
}
