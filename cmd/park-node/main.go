package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"p2p-park/internal/netx"
	parknode "p2p-park/internal/park-node"
)

func main() {
	name := flag.String("name", "anon", "display name")
	bind := flag.String("bind", ":0", "bind address (e.g. :0 for random port)")
	seed := flag.Bool("seed", false, "run as SeedNode (rendezvous/relay)")
	bootstrapStr := flag.String("bootstrap", "", "comma-separated bootstrap addresses host:port")
	debug := flag.Bool("debug", false, "enable debug logs")
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

	app, err := parknode.New(parknode.Config{
		Name:       *name,
		Bind:       *bind,
		IsSeed:     *seed,
		Bootstraps: bootstraps,
		Debug:      *debug,
	}, logger)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	if err := app.Start(); err != nil {
		log.Fatalf("start app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
