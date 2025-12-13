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
		Channel: "enc:" + chName,
		Body:    body,
	})
}
