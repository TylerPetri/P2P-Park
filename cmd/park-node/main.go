package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
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
	fmt.Println("	/quit		- exit")
	fmt.Println()

	// Reader for stdin.
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "/quit") {
				fmt.Println("quitting...")
				n.Stop()
				os.Exit(0)
			}
			if strings.HasPrefix(line, "/say ") {
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
				continue
			}
			fmt.Println("unknown command")
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
		if g.Channel != "global" {
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(g.Body, &body); err != nil {
			continue
		}
		fmt.Printf("[GOSSIP] %v\n", body)
	}
}
