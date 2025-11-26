package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"p2p-park/internal/app/points"
	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
	"strconv"
	"strings"
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

	logger := log.New(os.Stdout, "", log.LstdFlags)

	n, err := p2p.NewNode(p2p.NodeConfig{
		Name:       *name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   *bind,
		Bootstraps: bootstraps,
		Protocol:   "park-p2p/0.1.0",
		Logger:     logger,
	})
	if err != nil {
		log.Fatalf("create node: %v", err)
	}

	if err := n.Start(); err != nil {
		log.Fatalf("start node: %v", err)
	}

	fmt.Printf("Node started.\n")
	fmt.Printf("ID:		%s\n", n.ID())
	fmt.Printf("Addr:	%s\n\n", n.ListenAddr())
	fmt.Println("Commands:")
	fmt.Println("	/say <message>		- broadcast a chat-like message")
	fmt.Println("	/add <delta> 		- add points to yourself (e.g. /add 10)")
	fmt.Println("	/points		- show current scores")
	fmt.Println("	/quit		- exit")
	fmt.Println()

	id := n.Identity()
	pointsEngine := points.NewEngine(n.ID(), *name, id.Priv, id.Pub)

	broadcastSelfScore := func(snap proto.SignedPointsSnapshot) {
		body, _ := json.Marshal(snap)
		n.Broadcast(proto.Gossip{
			Channel: "points",
			Body:    body,
		})
	}

	if snap, err := pointsEngine.SnapshotSelf(); err != nil {
		broadcastSelfScore(snap)
	} else {
		fmt.Printf("failed to sign initial snapshot: %v\n", err)
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

			case strings.HasPrefix(line, "/say "):
				msg := strings.TrimSpace(strings.TrimPrefix(line, "/say"))
				body, _ := json.Marshal(map[string]any{
					"text":      msg,
					"from":      *name,
					"timestamp": time.Now().Unix(),
				})
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

		switch g.Channel {
		case "global":
			var body map[string]any
			if err := json.Unmarshal(g.Body, &body); err != nil {
				continue
			}
			fmt.Printf("[GOSSIP] %v\n", body)

		case "points":
			var signed proto.SignedPointsSnapshot
			if err := json.Unmarshal(g.Body, &signed); err != nil {
				continue
			}
			if pointsEngine.ApplyRemote(signed) {
				s := signed.Snapshot
				fmt.Printf("[POINTS] %s now has %d points (v%d)\n", s.Name, s.Points, s.Version)
			}
		}
	}
}
