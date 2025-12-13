package parknode

import (
	"encoding/json"
	"strings"
	"time"

	"p2p-park/internal/crypto/channel"
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
