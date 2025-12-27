package grants

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"time"

	"p2p-park/internal/proto"
)

var (
	ErrInvalidGrant = errors.New("invalid grant")
	ErrBadSignature = errors.New("bad grant signature")
)

// VerifyGrant performs deterministic, local validation of a QuizGrant.
// It does NOT check whether the grant was already seen (dedup is the caller's job).
func VerifyGrant(g proto.QuizGrant) error {
	if g.GrantID == "" || g.QuizID == "" || g.GrantorID == "" || g.RecipientID == "" {
		return ErrInvalidGrant
	}
	if g.Points == 0 {
		return ErrInvalidGrant
	}

	// expiry window: ignore absurd timestamps (avoids unbounded bbolt growth from garbage).
	now := time.Now().Unix()
	if g.Timestamp > now+60 || g.Timestamp < now-60*60*24*365 {
		return ErrInvalidGrant
	}

	pubBytes, err := hex.DecodeString(g.GrantorID)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return ErrInvalidGrant
	}
	msg, _ := proto.EncodeQuizGrantCanonical(g)
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), msg, g.Signature) {
		return ErrBadSignature
	}
	return nil
}
