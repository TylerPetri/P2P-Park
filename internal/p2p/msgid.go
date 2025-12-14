package p2p

import (
	"crypto/rand"
	"encoding/hex"
)

func NewMsgID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
