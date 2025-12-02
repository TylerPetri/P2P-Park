package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"p2p-park/internal/app/points"
	"p2p-park/internal/crypto/channel"
	"p2p-park/internal/discovery"
	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	name := flag.String("name", "anon", "display name")
	bind := flag.String("bind", ":0", "bind address (e.g. :0 for random port)")
	bootstrapStr := flag.String("bootstrap", "", "comma-separated bootstrap addresses host:port")
	flag.Parse()

	var bootstraps []netx.Addr
	if *bootstrapStr != "" {
		for _, part := range strings.Split(*bootstrapStr, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				bootstraps = append(bootstraps, netx.Addr(part))
			}
		}
	}

	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		close(stop)
	}()

	logger := log.New(os.Stdout, "", log.LstdFlags)

	n, err := p2p.NewNode(p2p.NodeConfig{
		Name:       *name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   *bind,
		Bootstraps: bootstraps,
		Protocol:   "park-p2p/0.1.0",
		Logger:     logger,
		Debug:      false,
	})
	if err != nil {
		log.Fatalf("create node: %v", err)
	}

	if err := n.Start(); err != nil {
		log.Fatalf("start node: %v", err)
	}

	// Start LAN discovery responder
	lanCfg := discovery.DefaultLANConfig()
	listenAddr := n.ListenAddr()
	if err := discovery.StartLANResponder(stop, lanCfg, string(listenAddr), *name); err != nil {
		fmt.Printf("LAN responder failed: %v\n", err)
	}

	// Start DiscoveryManager (LAN + PeerStore + Seeds)
	ps := discovery.NewPeerStore(discovery.DefaultPeerStorePath())
	mgr := discovery.NewManager(ps, lanCfg)
	mgr.Run(n)

	fmt.Printf("Node started.\n")
	fmt.Printf("ID:		%s\n", n.ID())
	fmt.Printf("Addr:	%s\n\n", n.ListenAddr())
	fmt.Println("Commands:")
	fmt.Println("	/say <message>		- broadcast a chat-like message")
	fmt.Println("	/add <delta> 		- add points to yourself")
	fmt.Println(" /me	-	prints your info")
	fmt.Println("	/points		- show current scores")
	fmt.Println("	/mkchan	<name>	- make an encrypted channel")
	fmt.Println("	/joinchan	<name> <hexkey>	- join an ecrypted channel")
	fmt.Println("	/encsay	<channel> <message>	- broadcast an encrypted message to a channel")
	fmt.Println("	/quit		- exit")
	fmt.Println()

	id := n.Identity()
	pointsEngine := points.NewEngine(*name, id.SignPriv, id.SignPub)

	broadcastSelfScore := func(snap proto.SignedPointsSnapshot) {
		body, _ := json.Marshal(snap)
		n.Broadcast(proto.Gossip{
			Channel: "points",
			Body:    body,
		})
	}

	if snap, err := pointsEngine.SnapshotSelf(); err != nil {
		fmt.Printf("failed to sign initial snapshot: %v\n", err)
	} else {
		broadcastSelfScore(snap)
	}

	encChannels := make(map[string]channel.ChannelKey)

	sendEncrypted := func(chName, msg string) {
		key, ok := encChannels[chName]
		if !ok {
			fmt.Printf("not joined to channel %q\n", chName)
			return
		}
		plaintext, _ := json.Marshal(map[string]any{
			"text":      msg,
			"from":      *name,
			"channel":   chName,
			"timestamp": time.Now().Unix(),
		})
		nonce, ct, err := channel.Encrypt(key, plaintext)
		if err != nil {
			fmt.Printf("encrypt failed: %v\n", err)
			return
		}
		em := proto.EncryptedMessage{
			Nonce:      nonce,
			Ciphertext: ct,
		}
		body, _ := json.Marshal(em)
		n.Broadcast(proto.Gossip{
			Channel: "enc:" + chName,
			Body:    body,
		})
	}

	// Reader for stdin.
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			switch {

			case strings.HasPrefix(line, "/quit"):
				fmt.Println("quitting...")
				n.Stop()
				os.Exit(0)

			case line == "/me":
				id := n.Identity()

				userID := hex.EncodeToString(id.SignPub) // ed25519
				networkID := id.ID                       // Noise hex ID

				fmt.Println()
				fmt.Println("== You ==")
				fmt.Printf("  Name:       %s\n", *name)
				fmt.Printf("  UserID:     %s\n", userID)
				fmt.Printf("  NetworkID:  %s\n", networkID)
				fmt.Printf("  Listen on:  %s\n", n.ListenAddr())
				fmt.Println()

			case line == "/peers":
				peers := n.SnapshotPeers()
				if len(peers) == 0 {
					fmt.Println("no peers connected")
					continue
				}

				fmt.Println()
				fmt.Println("Connected peers:")
				fmt.Printf("%-16s  %-10s  %-10s  %s\n", "NAME", "USERID", "NETID", "ADDR")
				fmt.Printf("%-16s  %-10s  %-10s  %s\n", "----", "------", "-----", "----")

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

					fmt.Printf("%-16s  %-10s  %-10s  %s\n",
						coloredName, shortUser, shortNet, p.Addr)
				}
				fmt.Println()

			case strings.HasPrefix(line, "/say "):
				msg := strings.TrimSpace(strings.TrimPrefix(line, "/say"))
				chat := proto.ChatMessage{
					Text:      msg,
					From:      *name,
					Timestamp: time.Now().Unix(),
				}
				body, _ := json.Marshal(chat)
				n.Broadcast(proto.Gossip{
					Channel: "global",
					Body:    body,
				})

			case strings.HasPrefix(line, "/add "):
				arg := strings.TrimSpace(strings.TrimPrefix(line, "/add"))
				if arg == "" {
					fmt.Println("usage: /add <delta>")
					continue
				}
				delta, err := strconv.ParseInt(arg, 10, 64)
				if err != nil {
					fmt.Printf("bad delta: %v\n", err)
					continue
				}
				signed, err := pointsEngine.AddSelf(delta)
				if err != nil {
					fmt.Printf("failed to sign snapshot: %v\n", err)
					continue
				}
				fmt.Printf("[POINTS] You now have %d points (v%d)\n", signed.Snapshot.Points, signed.Snapshot.Version)
				broadcastSelfScore(signed)

			case line == "/points":
				snaps := pointsEngine.All()
				fmt.Println("== Scores ==")
				for i, s := range snaps {
					shortID := s.PlayerID
					if len(shortID) > 8 {
						shortID = shortID[:8]
					}
					fmt.Printf("%2d. %-12s (%s) : %d\n", i+1, s.Name, shortID, s.Points)
				}

			case strings.HasPrefix(line, "/mkchan "):
				chName := strings.TrimSpace(strings.TrimPrefix(line, "/mkchan"))
				if chName == "" {
					fmt.Println("usage: /mkchan <name>")
					continue
				}
				k, err := channel.NewRandomKey()
				if err != nil {
					fmt.Printf("mkchan: %v\n", err)
					continue
				}
				encChannels[chName] = k
				fmt.Printf("[CHAN] created channel %q\n", chName)
				fmt.Printf("[CHAN] share this key with others:\n  %s\n", channel.KeyToHex(k))

			case strings.HasPrefix(line, "/joinchan "):
				parts := strings.Fields(line)
				if len(parts) != 3 {
					fmt.Println("usage: /joinchan <name> <hexkey>")
					continue
				}
				chName := parts[1]
				hexKey := parts[2]
				k, err := channel.ParseKeyHex(hexKey)
				if err != nil {
					fmt.Printf("joinchan: %v\n", err)
					continue
				}
				encChannels[chName] = k
				fmt.Printf("[CHAN] joined channel %q\n", chName)

			case strings.HasPrefix(line, "/encsay "):
				rest := strings.TrimSpace(strings.TrimPrefix(line, "/encsay"))
				parts := strings.Fields(rest)
				if len(parts) < 2 {
					fmt.Println("usage: /encsay <channel> <message>")
					continue
				}
				chName := parts[0]
				msg := strings.TrimSpace(strings.TrimPrefix(rest, chName))
				sendEncrypted(chName, msg)

			default:
				fmt.Println("unknown command")
			}
		}
	}()

	// Receive loop for gossip messages.
	for env := range n.Incoming() {
		if env.Type != proto.MsgGossip {
			continue
		}
		var g proto.Gossip
		if err := json.Unmarshal(env.Payload, &g); err != nil {
			continue
		}

		switch {
		case g.Channel == "global":
			var chat proto.ChatMessage
			if err := json.Unmarshal(g.Body, &chat); err != nil {
				fmt.Printf("[CHAT] bad chat payload: %v\n", err)
				continue
			}

			ts := time.Unix(chat.Timestamp, 0).Format("12:02:02")

			sender := n.PeerDisplayName(env.FromID)
			if sender == "" {
				sender = shortID(env.FromID)
			}
			colored := formatName(sender, env.FromID)

			fmt.Printf("%s[%s]%s %s: %s\n", ansiDim, ts, ansiReset, colored, chat.Text)

		case g.Channel == "points":
			var signed proto.SignedPointsSnapshot
			if err := json.Unmarshal(g.Body, &signed); err != nil {
				continue
			}
			if pointsEngine.ApplyRemote(signed) {
				s := signed.Snapshot
				fmt.Printf("[POINTS] %s now has %d points (v%d)\n", s.Name, s.Points, s.Version)
			}

		case strings.HasPrefix(g.Channel, "enc:"):
			chName := strings.TrimPrefix(g.Channel, "enc:")
			key, ok := encChannels[chName]
			if !ok {
				continue
			}
			var em proto.EncryptedMessage
			if err := json.Unmarshal(g.Body, &em); err != nil {
				continue
			}
			pt, err := channel.Decrypt(key, em.Nonce, em.Ciphertext)
			if err != nil {
				fmt.Printf("[ENC:%s] decrypt failed: %v\n", chName, err)
				continue
			}
			var body map[string]any
			if err := json.Unmarshal(pt, &body); err != nil {
				fmt.Printf("[ENC:%s] bad plaintext: %v\n", chName, err)
				continue
			}
			fmt.Printf("[ENC:%s] %v\n", chName, body)

		default:
		}
	}
}
