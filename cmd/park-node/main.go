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

	pointsEngine := points.NewEngine(n.ID(), *name)

	broadcastSelfScore := func(snap proto.PointsSnapshot) {
		body, _ := json.Marshal(snap)
		n.Broadcast(proto.Gossip{
			Channel: "points",
			Body:    body,
		})
	}

	broadcastSelfScore(pointsEngine.SnapshotSelf())

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
				snap := pointsEngine.AddSelf(delta)
				fmt.Printf("[POINTS] You now have %d points (v%d)\n", snap.Points, snap.Version)
				broadcastSelfScore(snap)
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
			var snap proto.PointsSnapshot
			if err := json.Unmarshal(g.Body, &snap); err != nil {
				continue
			}
			if pointsEngine.ApplyRemote(snap) {
				fmt.Printf("[POINTS] %s now has %d points (v%d)\n", snap.Name, snap.Points, snap.Version)
			}
		}
	}
}
