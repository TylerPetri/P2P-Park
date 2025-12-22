package dht

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const NodeIDBytes = 32

type NodeID [NodeIDBytes]byte

var ErrBadPeerID = errors.New("dht: bad peer id")

// NodeIDFromPeerID derives the routing identity from the transport identity.
// peerIDHex is hex(ed25519 pubkey), 32 bytes => 64 hex chars.
func NodeIDFromPeerID(peerIDHex string) (NodeID, error) {
	var out NodeID
	pub, err := hex.DecodeString(peerIDHex)
	if err != nil || len(pub) != 32 {
		return out, ErrBadPeerID
	}
	sum := sha256.Sum256(pub)
	copy(out[:], sum[:])
	return out, nil
}

// NodeIDMatchesPeerID verifies nodeIDHex == sha256(decode(peerIDHex)).
func NodeIDMatchesPeerID(nodeIDHex, peerIDHex string) bool {
	claimed, err := ParseNodeIDHex(nodeIDHex)
	if err != nil {
		return false
	}
	derived, err := NodeIDFromPeerID(peerIDHex)
	if err != nil {
		return false
	}
	return claimed == derived
}

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

func (id NodeID) Hex() string     { return hex.EncodeToString(id[:]) }
func Distance(a, b NodeID) NodeID { return Xor(a, b) }

func DistanceLess(a, b NodeID) bool {
	return bytes.Compare(a[:], b[:]) < 0
}

// XOR distance: d = a ^ b
func Xor(a, b NodeID) (out NodeID) {
	for i := 0; i < NodeIDBytes; i++ {
		out[i] = a[i] ^ b[i]
	}
	return
}

// BucketIndex returns [0..255] for 256-bit IDs.
func BucketIndex(self, other NodeID) int {
	d := Xor(self, other)
	for byteIdx := 0; byteIdx < NodeIDBytes; byteIdx++ {
		x := d[byteIdx]
		if x == 0 {
			continue
		}
		for bit := 0; bit < 8; bit++ {
			if x&(1<<(7-bit)) != 0 {
				return byteIdx*8 + bit
			}
		}
	}
	return -1
}

func RandomNodeID() NodeID {
	var id NodeID
	_, _ = rand.Read(id[:])
	return id
}
