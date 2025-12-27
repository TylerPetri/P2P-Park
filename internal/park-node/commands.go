package parknode

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"p2p-park/internal/crypto/channel"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
)

func (a *App) readStdin(_ any) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		a.handleCommand(line)
	}
}

func (a *App) handleCommand(line string) {
	switch {
	case strings.HasPrefix(line, "/quit"), strings.HasPrefix(line, "/exit"):
		a.ui.Println("quitting...")
		a.StopAll()
		sleepBrief()
		os.Exit(0)

	case line == "/me":
		id := a.Node.Identity()
		userID := hex.EncodeToString(id.SignPub) // ed25519 pubkey hex
		networkID := id.ID                       // Noise hex ID

		a.ui.Println()
		a.ui.Println("== You ==")
		a.ui.Printf("  Name:       %s\n", a.Node.Name())
		a.ui.Printf("  UserID:     %s\n", userID)
		a.ui.Printf("  NetworkID:  %s\n", networkID)
		a.ui.Printf("  Listen on:  %s\n", a.Node.ListenAddr())
		a.ui.Printf("  Peers:      %d\n", a.Node.PeerCount())
		a.ui.Println()

	case line == "/peers":
		peers := a.Node.SnapshotPeers()
		if len(peers) == 0 {
			a.ui.Println("no peers connected")
			return
		}

		a.ui.Println()
		a.ui.Println("Connected peers:")
		a.ui.Printf("%-16s  %-10s  %-10s  %s\n", "NAME", "USERID", "NETID", "ADDR")
		a.ui.Printf("%-16s  %-10s  %-10s  %s\n", "----", "------", "-----", "----")

		for _, p := range peers {
			name := p.Name
			if name == "" {
				name = shortID(p.NetworkID)
			}
			coloredName := formatName(name, p.NetworkID)

			shortUser := "-"
			if p.UserID != "" {
				shortUser = shortID(p.UserID)
			}
			shortNet := shortID(p.NetworkID)

			a.ui.Printf("%-16s  %-10s  %-10s  %s\n", coloredName, shortUser, shortNet, p.Addr)
		}
		a.ui.Println()

	case strings.HasPrefix(line, "/say "):
		msg := strings.TrimSpace(strings.TrimPrefix(line, "/say"))
		chat := proto.ChatMessage{
			Text:      msg,
			From:      a.Node.Name(),
			Timestamp: time.Now().Unix(),
		}
		body, _ := json.Marshal(chat)
		a.Node.Broadcast(proto.Gossip{
			ID:      p2p.NewMsgID(),
			Channel: "global",
			Body:    body,
		})

	case strings.HasPrefix(line, "/add "):
		arg := strings.TrimSpace(strings.TrimPrefix(line, "/add"))
		if arg == "" {
			a.ui.Println("usage: /add <delta>")
			return
		}
		delta, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			a.ui.Printf("bad delta: %v\n", err)
			return
		}
		signed, err := a.Points.AddSelf(delta)
		if err != nil {
			a.ui.Printf("failed to sign snapshot: %v\n", err)
			return
		}
		a.ui.Printf("[POINTS] You now have %d points (v%d)\n", signed.Snapshot.Points, signed.Snapshot.Version)
		a.broadcastPoints(signed)

	case line == "/points":
		snaps := a.Points.All()
		a.ui.Println("== Scores ==")
		for i, s := range snaps {
			pid := s.PlayerID
			if len(pid) > 8 {
				pid = pid[:8]
			}
			a.ui.Printf("%2d. %-12s (%s) : %d\n", i+1, s.Name, pid, s.Points)
		}

	case line == "/lb":
		// Grant-based leaderboard
		self := a.userIDHex()
		a.Ledger.NoteName(self, a.Node.Name())
		lb := a.Ledger.Leaderboard()
		a.ui.Println("== Leaderboard (grants) ==")
		if len(lb) == 0 {
			a.ui.Println("no grants yet")
			return
		}
		for i, s := range lb {
			pid := s.PlayerID
			if len(pid) > 8 {
				pid = pid[:8]
			}
			name := s.Name
			if name == "" {
				name = shortID(s.PlayerID)
			}
			a.ui.Printf("%2d. %-12s (%s) : %d\n", i+1, name, pid, s.Points)
		}

	case strings.HasPrefix(line, "/quizask "):
		// Format: /quizask <pts> <ttl_s> <question> | <answer>
		rest := strings.TrimSpace(strings.TrimPrefix(line, "/quizask"))
		// split answer by '|'
		parts := strings.SplitN(rest, "|", 2)
		if len(parts) != 2 {
			a.ui.Println("usage: /quizask <pts> <ttl_s> <question> | <answer>")
			return
		}
		left := strings.TrimSpace(parts[0])
		answer := strings.TrimSpace(parts[1])
		fields := strings.Fields(left)
		if len(fields) < 3 {
			a.ui.Println("usage: /quizask <pts> <ttl_s> <question> | <answer>")
			return
		}
		pts, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			a.ui.Printf("bad pts: %v\n", err)
			return
		}
		ttlS, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			a.ui.Printf("bad ttl_s: %v\n", err)
			return
		}
		question := strings.TrimSpace(strings.TrimPrefix(left, fields[0]+" "+fields[1]))
		if question == "" || answer == "" {
			a.ui.Println("question and answer required")
			return
		}
		signed, qid, err := a.Quiz.CreateOpen(question, answer, pts, time.Duration(ttlS)*time.Second)
		if err != nil {
			a.ui.Printf("quizask failed: %v\n", err)
			return
		}
		wire := proto.QuizWire{Kind: "open", Open: &signed}
		body, _ := json.Marshal(wire)
		a.Node.Broadcast(proto.Gossip{ID: p2p.NewMsgID(), Channel: "quiz", Body: body})
		a.ui.Printf("[QUIZ] opened %s (%d pts, ttl=%ds): %s\n", shortID(qid), pts, ttlS, question)

	case line == "/quizzes":
		opens := a.Quiz.ListOpen()
		if len(opens) == 0 {
			a.ui.Println("no open quizzes")
			return
		}
		a.ui.Println("== Open Quizzes ==")
		now := time.Now().Unix()
		for _, o := range opens {
			exp := "-"
			if o.Expires != 0 {
				if now > o.Expires {
					exp = "expired"
				} else {
					exp = time.Unix(o.Expires, 0).Format("15:04:05")
				}
			}
			a.ui.Printf("%s  %d pts  by %s  exp:%s\n    %s\n", shortID(o.QuizID), o.Points, shortID(o.CreatorID), exp, o.Question)
		}

	case strings.HasPrefix(line, "/quizanswer "):
		rest := strings.TrimSpace(strings.TrimPrefix(line, "/quizanswer"))
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			a.ui.Println("usage: /quizanswer <quiz_id> <answer>")
			return
		}
		qid := fields[0]
		// Expand abbreviated quiz IDs (shown in /quizzes)
		if full, ok := a.Quiz.ResolveQuizID(qid); ok {
			qid = full
		} else {
			a.ui.Printf("[QUIZ] unknown or ambiguous quiz id: %s\n", qid)
			return
		}
		answer := strings.TrimSpace(strings.Join(fields[1:], " "))
		wire := proto.QuizWire{Kind: "answer", Answer: &proto.QuizAnswer{QuizID: qid, Answer: answer, Timestamp: time.Now().Unix()}}
		body, _ := json.Marshal(wire)
		a.Node.Broadcast(proto.Gossip{ID: p2p.NewMsgID(), Channel: "quiz", Body: body})
		a.ui.Printf("[QUIZ] answered %s\n", shortID(qid))

	case strings.HasPrefix(line, "/mkchan "):
		chName := strings.TrimSpace(strings.TrimPrefix(line, "/mkchan"))
		if chName == "" {
			a.ui.Println("usage: /mkchan <name>")
			return
		}
		k, err := channel.NewRandomKey()
		if err != nil {
			a.ui.Printf("mkchan: %v\n", err)
			return
		}
		a.encMu.Lock()
		a.encChannels[chName] = k
		a.encMu.Unlock()

		a.ui.Printf("[CHAN] created channel %q\n", chName)
		a.ui.Printf("[CHAN] share this key with others:\n  %s\n", channel.KeyToHex(k))

	case strings.HasPrefix(line, "/joinchan "):
		parts := strings.Fields(line)
		if len(parts) != 3 {
			a.ui.Println("usage: /joinchan <name> <hexkey>")
			return
		}
		chName := parts[1]
		hexKey := parts[2]

		k, err := channel.ParseKeyHex(hexKey)
		if err != nil {
			a.ui.Printf("joinchan: %v\n", err)
			return
		}
		a.encMu.Lock()
		a.encChannels[chName] = k
		a.encMu.Unlock()

		a.ui.Printf("[CHAN] joined channel %q\n", chName)

	case strings.HasPrefix(line, "/encsay "):
		rest := strings.TrimSpace(strings.TrimPrefix(line, "/encsay"))
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			a.ui.Println("usage: /encsay <channel> <message>")
			return
		}
		chName := parts[0]
		msg := strings.TrimSpace(strings.TrimPrefix(rest, chName))

		a.sendEncrypted(chName, msg)

	default:
		a.ui.Println("unknown command")
		PrintCommands(a.ui)
	}
}

func (a *App) sendEncrypted(chName, msg string) {
	a.encMu.RLock()
	key, ok := a.encChannels[chName]
	a.encMu.RUnlock()
	if !ok {
		a.ui.Printf("not joined to channel %q\n", chName)
		return
	}

	plaintext, _ := json.Marshal(map[string]any{
		"text":      msg,
		"from":      a.Node.Name(),
		"channel":   chName,
		"timestamp": time.Now().Unix(),
	})

	nonce, ct, err := channel.Encrypt(key, plaintext)
	if err != nil {
		a.ui.Printf("encrypt failed: %v\n", err)
		return
	}

	em := proto.EncryptedMessage{
		Nonce:      nonce,
		Ciphertext: ct,
	}

	body, _ := json.Marshal(em)
	a.Node.Broadcast(proto.Gossip{
		ID:      p2p.NewMsgID(),
		Channel: "enc:" + chName,
		Body:    body,
	})
}
