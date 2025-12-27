package parknode

import (
	"encoding/json"
	"strings"
	"time"

	"p2p-park/internal/crypto/channel"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
)

func (a *App) handleEnvelope(env proto.Envelope) {
	if env.Type != proto.MsgGossip {
		return
	}

	var g proto.Gossip
	if err := json.Unmarshal(env.Payload, &g); err != nil {
		return
	}

	switch {
	case strings.HasPrefix(g.Channel, "enc:"):
		a.handleEncrypted(env, g)
		return

	case g.Channel == "global":
		a.handleGlobalChat(env, g)
		return

	case g.Channel == "points":
		a.handlePoints(g)
		return

	case g.Channel == "quiz":
		a.handleQuiz(env, g)
		return

	default:
		return
	}
}

func (a *App) handleGlobalChat(env proto.Envelope, g proto.Gossip) {
	var chat proto.ChatMessage
	if err := json.Unmarshal(g.Body, &chat); err != nil {
		a.ui.Printf("[CHAT] bad chat payload: %v\n", err)
		return
	}

	ts := time.Unix(chat.Timestamp, 0).Format("15:04:05")

	sender := a.Node.PeerDisplayName(env.FromID)
	if sender == "" {
		sender = shortID(env.FromID)
	}
	colored := formatName(sender, env.FromID)

	a.ui.Printf("%s[%s]%s %s: %s\n", ansiDim, ts, ansiReset, colored, chat.Text)
}

func (a *App) handlePoints(g proto.Gossip) {
	var signed proto.SignedPointsSnapshot
	if err := json.Unmarshal(g.Body, &signed); err != nil {
		a.ui.Printf("[POINTS] bad snapshot: %v\n", err)
		return
	}
	_ = a.Points.ApplyRemote(signed)
}

func (a *App) handleQuiz(env proto.Envelope, g proto.Gossip) {
	var qw proto.QuizWire
	if err := json.Unmarshal(g.Body, &qw); err != nil {
		a.ui.Printf("[QUIZ] bad payload: %v\n", err)
		return
	}

	switch qw.Kind {
	case "open":
		if qw.Open == nil {
			return
		}
		if a.Quiz.ObserveOpen(*qw.Open) {
			// Best-effort name learning
			creator := qw.Open.Open.CreatorID
			a.Ledger.NoteName(creator, a.Node.PeerDisplayName(env.FromID))
			a.ui.Printf("[QUIZ] open %s (%d pts): %s\n", shortID(qw.Open.Open.QuizID), qw.Open.Open.Points, qw.Open.Open.Question)
		}

	case "answer":
		if qw.Answer == nil {
			return
		}
		ans := *qw.Answer

		answererUserID := a.Node.UserIDForPeer(env.FromID)
		if answererUserID == "" {
			return
		}

		grant, correct, authoritative, already := a.Quiz.TryGrade(ans.QuizID, answererUserID, ans.Answer)
		_ = already
		if !authoritative {
			return
		}

		// direct UX result (unsigned)
		res := proto.QuizResult{QuizID: ans.QuizID, PlayerID: answererUserID, Correct: correct, Delta: 0}
		if correct {
			res.Delta = grant.Points
		}
		resWire := proto.QuizWire{Kind: "result", Result: &res}
		resBody, _ := json.Marshal(resWire)
		resG := proto.Gossip{ID: p2p.NewMsgID(), Channel: "quiz", Body: resBody}

		_ = a.Node.SendToPeer(env.FromID, proto.Envelope{
			Type:    proto.MsgGossip,
			FromID:  a.Node.ID(),
			Payload: proto.MustMarshal(resG),
		})

		if correct {
			grantWire := proto.QuizWire{Kind: "grant", Grant: &grant}
			grantBody, _ := json.Marshal(grantWire)
			a.Node.Broadcast(proto.Gossip{ID: grant.GrantID, Channel: "quiz", Body: grantBody})
		}

	case "grant":
		if qw.Grant == nil {
			return
		}
		g := *qw.Grant

		// ApplyGrant is your dedup gate. Only relay if this is the first time we've seen it.
		if a.Ledger.ApplyGrant(g) {
			// Relay with stable ID so the whole network converges.
			// Use the original body we just parsed so we don't re-encode inconsistently.
			// (But ensure Gossip.ID is stable.)

			wire := proto.QuizWire{Kind: "grant", Grant: &g}
			body, _ := json.Marshal(wire)
			relay := proto.Gossip{ID: g.GrantID, Channel: "quiz", Body: body}

			a.Node.Broadcast(relay)

			// announce if it's about us
			if g.RecipientID == a.userIDHex() {
				a.ui.Printf("[POINTS] +%d (quiz grant) => total %d\n", g.Points, a.Ledger.Total(g.RecipientID))
			}
		}

	case "result":
		if qw.Result == nil {
			return
		}
		if qw.Result.PlayerID != a.userIDHex() {
			return
		}
		if qw.Result.Correct {
			a.ui.Printf("[QUIZ] correct! +%d points (quiz %s)\n", qw.Result.Delta, shortID(qw.Result.QuizID))
		} else {
			a.ui.Printf("[QUIZ] incorrect (quiz %s)\n", shortID(qw.Result.QuizID))
		}
	}
}

func (a *App) handleEncrypted(env proto.Envelope, g proto.Gossip) {
	chName := strings.TrimPrefix(g.Channel, "enc:")

	a.encMu.RLock()
	key, ok := a.encChannels[chName]
	a.encMu.RUnlock()
	if !ok {
		// Not joined; ignore.
		return
	}

	var em proto.EncryptedMessage
	if err := json.Unmarshal(g.Body, &em); err != nil {
		a.ui.Printf("[enc:%s] bad encrypted payload: %v\n", chName, err)
		return
	}

	pt, err := channel.Decrypt(key, em.Nonce, em.Ciphertext)
	if err != nil {
		a.ui.Printf("[enc:%s] decrypt failed: %v\n", chName, err)
		return
	}

	// plaintext is a JSON object we created in sendEncrypted()
	var payload map[string]any
	if err := json.Unmarshal(pt, &payload); err != nil {
		a.ui.Printf("[enc:%s] %s\n", chName, string(pt))
		return
	}

	from, _ := payload["from"].(string)
	text, _ := payload["text"].(string)

	if from == "" {
		from = a.Node.PeerDisplayName(env.FromID)
		if from == "" {
			from = shortID(env.FromID)
		}
	}

	a.ui.Printf("[enc:%s] %s: %s\n", chName, from, text)
}
