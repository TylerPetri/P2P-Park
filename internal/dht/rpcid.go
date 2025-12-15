package dht

import (
	"crypto/rand"
	"encoding/hex"
)

func newRPCID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
