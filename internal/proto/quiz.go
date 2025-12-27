package proto

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
)

// QuizWire is carried inside Gossip.Body for channel "quiz".
// It is intentionally simple and explicit.
type QuizWire struct {
	Kind   string          `json:"kind"` // "open" | "answer" | "grant" | "result"
	Open   *QuizOpenSigned `json:"open,omitempty"`
	Answer *QuizAnswer     `json:"answer,omitempty"`
	Grant  *QuizGrant      `json:"grant,omitempty"`
	Result *QuizResult     `json:"result,omitempty"`
}

// QuizOpen is the public announcement of a quiz.
// The correct answer is NOT included.
type QuizOpen struct {
	QuizID    string `json:"quiz_id"`
	CreatorID string `json:"creator_id"` // PeerID (hex ed25519 pub)
	Question  string `json:"question"`
	Points    int64  `json:"points"`
	Created   int64  `json:"created"`
	Expires   int64  `json:"expires"`
}

// QuizOpenSigned binds the open announcement to the creator.
type QuizOpenSigned struct {
	Open      QuizOpen `json:"open"`
	Signature []byte   `json:"sig"`
}

// QuizAnswer is broadcast for now (Phase 6+ can make this private).
type QuizAnswer struct {
	QuizID    string `json:"quiz_id"`
	Answer    string `json:"answer"`
	Timestamp int64  `json:"ts"`
}

// QuizGrant is the signed points award. Everyone can verify the signature.
type QuizGrant struct {
	GrantID     string `json:"grant_id"` // unique id (msg id)
	QuizID      string `json:"quiz_id"`
	GrantorID   string `json:"grantor_id"`   // PeerID (hex ed25519 pub)
	RecipientID string `json:"recipient_id"` // PeerID (hex ed25519 pub)
	Points      int64  `json:"points"`
	Timestamp   int64  `json:"ts"`
	Signature   []byte `json:"sig"`
}

type QuizResult struct {
	QuizID   string `json:"quiz_id"`
	PlayerID string `json:"player_id"` // UserID (ed25519 hex)
	Correct  bool   `json:"correct"`
	Delta    int64  `json:"delta"` // points awarded (0 if incorrect)
}

func EncodeQuizOpenCanonical(o QuizOpen) ([]byte, error) {
	// Stable JSON is fine here (we only sign our own payloads).
	// For maximum determinism, we build a struct and marshal.
	return json.Marshal(o)
}

func EncodeQuizGrantCanonical(g QuizGrant) ([]byte, error) {
	// Canonical bytes for signing/verification:
	// sha256( grant_id || quiz_id || grantor_id || recipient_id || points || ts )
	buf := make([]byte, 0, len(g.GrantID)+len(g.QuizID)+len(g.GrantorID)+len(g.RecipientID)+16)
	buf = append(buf, []byte(g.GrantID)...)
	buf = append(buf, 0)
	buf = append(buf, []byte(g.QuizID)...)
	buf = append(buf, 0)
	buf = append(buf, []byte(g.GrantorID)...)
	buf = append(buf, 0)
	buf = append(buf, []byte(g.RecipientID)...)
	buf = append(buf, 0)
	pt := make([]byte, 8)
	binary.BigEndian.PutUint64(pt, uint64(g.Points))
	buf = append(buf, pt...)
	tt := make([]byte, 8)
	binary.BigEndian.PutUint64(tt, uint64(g.Timestamp))
	buf = append(buf, tt...)
	sum := sha256.Sum256(buf)
	return sum[:], nil
}

func DecodePeerIDHexToPub(peerID string) ([]byte, bool) {
	b, err := hex.DecodeString(peerID)
	if err != nil || len(b) != 32 {
		return nil, false
	}
	return b, true
}
