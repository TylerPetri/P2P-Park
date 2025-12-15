package dht

import (
	"encoding/hex"
	"fmt"
)

const NodeIDBytes = 32

type NodeID [NodeIDBytes]byte

func ParseNodeIDHex(s string) (NodeID, error) {
	var id NodeID
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, err
	}
	if len(b) != NodeIDBytes {
		return id, fmt.Errorf("node id must be %d bytes, got %d", NodeIDBytes, len(b))
	}
	copy(id[:], b)
	return id, nil
}

func MustParseNodeIDHex(s string) NodeID {
	id, err := ParseNodeIDHex(s)
	if err != nil {
		panic(err)
	}
	return id
}

func (id NodeID) Hex() string { return hex.EncodeToString(id[:]) }

// XOR distance: d = a ^ b
func Xor(a, b NodeID) (out NodeID) {
	for i := 0; i < NodeIDBytes; i++ {
		out[i] = a[i] ^ b[i]
	}
	return
}

// BucketIndex returns [0..255] for 256-bit IDs.
// It’s the index of the first differing bit (MSB-first).
// If identical, returns -1.
func BucketIndex(self, other NodeID) int {
	d := Xor(self, other)
	for byteIdx := 0; byteIdx < NodeIDBytes; byteIdx++ {
		x := d[byteIdx]
		if x == 0 {
			continue
		}
		// find first set bit in this byte (MSB first)
		for bit := 0; bit < 8; bit++ {
			if x&(1<<(7-bit)) != 0 {
				return byteIdx*8 + bit
			}
		}
	}
	return -1
}
