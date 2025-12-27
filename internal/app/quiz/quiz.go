package quiz

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
)

type Open struct {
	QuizID    string
	CreatorID string
	Question  string
	Points    int64
	Created   int64
	Expires   int64
}

type localOpen struct {
	open       Open
	answerHash [32]byte
	awardedTo  map[string]bool
}

// Engine tracks open quizzes and can create/verify grants.
// It is intentionally authority-based: the quiz creator is the authority for grading.
type Engine struct {
	mu sync.RWMutex

	selfPeerID string
	selfName   string
	priv       ed25519.PrivateKey
	pub        ed25519.PublicKey

	// opens holds public opens we've seen (including ours)
	opens map[string]Open
	// local holds quizzes we created (includes answer)
	local map[string]*localOpen
}

func NewEngine(selfName string, priv ed25519.PrivateKey, pub ed25519.PublicKey) *Engine {
	return &Engine{
		selfPeerID: p2p.PlayerIDFromPub(pub),
		selfName:   selfName,
		priv:       priv,
		pub:        pub,
		opens:      make(map[string]Open),
		local:      make(map[string]*localOpen),
	}
}

func normalizeAnswer(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func hashAnswer(a string) [32]byte {
	return sha256.Sum256([]byte(normalizeAnswer(a)))
}

// CreateOpen creates a new quiz open locally. The correct answer is stored locally only.
func (e *Engine) CreateOpen(question, answer string, points int64, ttl time.Duration) (proto.QuizOpenSigned, string, error) {
	qid := p2p.NewMsgID()
	now := time.Now()
	exp := int64(0)
	if ttl > 0 {
		exp = now.Add(ttl).Unix()
	}

	o := proto.QuizOpen{
		QuizID:    qid,
		CreatorID: e.selfPeerID,
		Question:  question,
		Points:    points,
		Created:   now.Unix(),
		Expires:   exp,
	}

	data, err := proto.EncodeQuizOpenCanonical(o)
	if err != nil {
		return proto.QuizOpenSigned{}, "", err
	}
	sig := ed25519.Sign(e.priv, data)
	signed := proto.QuizOpenSigned{Open: o, Signature: sig}

	e.mu.Lock()
	e.opens[qid] = Open{QuizID: qid, CreatorID: o.CreatorID, Question: o.Question, Points: o.Points, Created: o.Created, Expires: o.Expires}
	e.local[qid] = &localOpen{open: e.opens[qid], answerHash: hashAnswer(answer), awardedTo: make(map[string]bool)}
	e.mu.Unlock()

	return signed, qid, nil
}

// ObserveOpen validates a signed open and stores it.
func (e *Engine) ObserveOpen(s proto.QuizOpenSigned) bool {
	if s.Open.QuizID == "" || s.Open.CreatorID == "" || s.Open.Question == "" {
		return false
	}
	pubBytes, err := hex.DecodeString(s.Open.CreatorID)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	data, err := proto.EncodeQuizOpenCanonical(s.Open)
	if err != nil {
		return false
	}
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), data, s.Signature) {
		return false
	}
	now := time.Now().Unix()
	if s.Open.Expires != 0 && now > s.Open.Expires {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.opens[s.Open.QuizID]; ok {
		return false
	}
	e.opens[s.Open.QuizID] = Open{QuizID: s.Open.QuizID, CreatorID: s.Open.CreatorID, Question: s.Open.Question, Points: s.Open.Points, Created: s.Open.Created, Expires: s.Open.Expires}
	return true
}

// ListOpen returns the currently known open quizzes.
func (e *Engine) ListOpen() []Open {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Open, 0, len(e.opens))
	for _, o := range e.opens {
		out = append(out, o)
	}
	return out
}

func (e *Engine) GradeAnswer(quizID, answererPeerID, answer string) (proto.QuizGrant, bool) {
	g, correct, authoritative, already := e.TryGrade(quizID, answererPeerID, answer)
	_ = authoritative
	_ = already
	if !correct {
		return proto.QuizGrant{}, false
	}
	return g, true
}

// TryGrade evaluates an answer for a quiz we created.
// Returns:
//   - authoritative=false if we are not the creator / don't have the answer locally.
//   - already=true if we've already awarded this recipient for this quiz.
//   - correct=true if the answer matches.
//   - grant is populated only when correct==true.
func (e *Engine) TryGrade(quizID, answererID, answer string) (grant proto.QuizGrant, correct bool, authoritative bool, already bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	lo := e.local[quizID]
	if lo == nil {
		return proto.QuizGrant{}, false, false, false
	}
	authoritative = true

	now := time.Now().Unix()
	if lo.open.Expires != 0 && now > lo.open.Expires {
		return proto.QuizGrant{}, false, true, false
	}
	if lo.awardedTo[answererID] {
		return proto.QuizGrant{}, false, true, true
	}
	if hashAnswer(answer) != lo.answerHash {
		return proto.QuizGrant{}, false, true, false
	}

	grant = proto.QuizGrant{
		GrantID:     p2p.NewMsgID(),
		QuizID:      quizID,
		GrantorID:   e.selfPeerID,
		RecipientID: answererID,
		Points:      lo.open.Points,
		Timestamp:   now,
	}
	msg, _ := proto.EncodeQuizGrantCanonical(grant)
	grant.Signature = ed25519.Sign(e.priv, msg)
	lo.awardedTo[answererID] = true
	return grant, true, true, false
}

// ResolveQuizID expands a user-provided prefix into a full quiz ID.
// It returns (fullID,true) if the prefix matches exactly one known quiz,
// otherwise ("",false).
func (e *Engine) ResolveQuizID(prefix string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if prefix == "" {
		return "", false
	}
	// Exact match fast path
	if _, ok := e.opens[prefix]; ok {
		return prefix, true
	}
	if _, ok := e.local[prefix]; ok {
		return prefix, true
	}
	var match string
	// Search opens and locals
	for id := range e.opens {
		if strings.HasPrefix(id, prefix) {
			if match != "" && match != id {
				return "", false
			}
			match = id
		}
	}
	for id := range e.local {
		if strings.HasPrefix(id, prefix) {
			if match != "" && match != id {
				return "", false
			}
			match = id
		}
	}
	if match == "" {
		return "", false
	}
	return match, true
}
