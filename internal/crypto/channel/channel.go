package channel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// ChannelKey is a 32-byte symmetric key for a channel.
type ChannelKey [32]byte

// NewRandomKey generates a new random channel key.
func NewRandomKey() (ChannelKey, error) {
	var k ChannelKey
	if _, err := io.ReadFull(rand.Reader, k[:]); err != nil {
		return ChannelKey{}, err
	}
	return k, nil
}

// KeyToHex encodes a key as a hex string for sharing.
func KeyToHex(k ChannelKey) string {
	return hex.EncodeToString(k[:])
}

// ParseKeyHex parses a hex string into a ChannelKey.
func ParseKeyHex(s string) (ChannelKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return ChannelKey{}, err
	}
	if len(b) != 32 {
		return ChannelKey{}, fmt.Errorf("expected 32-byte key, got %d", len(b))
	}
	var k ChannelKey
	copy(k[:], b)
	return k, nil
}

// Encrypt encrypts plaintext using XChaCha20-Poly1305 with the given key.
func Encrypt(key ChannelKey, plaintext []byte) ([]byte, []byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct := aead.Seal(nil, nonce, plaintext, nil)
	return nonce, ct, nil
}

// Decrypt decrypts ciphertext using XChaCha20-Poly1305 with the given key.
func Decrypt(key ChannelKey, nonce, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, err
	}
	pt, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}
