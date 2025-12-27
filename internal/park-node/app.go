package parknode

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"p2p-park/internal/app/grants"
	"p2p-park/internal/app/points"
	"p2p-park/internal/app/quiz"
	"p2p-park/internal/crypto/channel"
	"p2p-park/internal/discovery"
	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
	"p2p-park/internal/proto"
	"p2p-park/internal/telemetry"
)

type App struct {
	cfg    Config
	ui     Printer
	logger telemetry.Logger

	Node *p2p.Node

	// Discovery lifecycle
	stopLAN chan struct{}

	// Points engine
	Points *points.Engine

	// Grant-based scoring (quiz awards)
	Ledger *grants.Ledger

	// Quiz engine
	Quiz *quiz.Engine

	// Encrypted channels
	encMu       sync.RWMutex
	encChannels map[string]channel.ChannelKey

	// Points cache (other users)
	pointsMu    sync.RWMutex
	otherPoints map[string]proto.PointsSnapshot
}

func New(cfg Config, logger *log.Logger) (*App, error) {
	n, err := p2p.NewNode(p2p.NodeConfig{
		Name:       cfg.Name,
		Network:    netx.NewTCPNetwork(),
		BindAddr:   cfg.Bind,
		Bootstraps: cfg.Bootstraps,
		Protocol:   "park-p2p/0.1.0",
		Logger:     logger,
		Debug:      cfg.Debug,
		IsSeed:     cfg.IsSeed,
	})
	if err != nil {
		return nil, err
	}

	id := n.Identity()
	pe := points.NewEngine(cfg.Name, id.SignPriv, id.SignPub)
	qe := quiz.NewEngine(cfg.Name, id.SignPriv, id.SignPub)
	ld := grants.NewLedger()
	ld.NoteName(hex.EncodeToString(id.SignPub), cfg.Name)

	return &App{
		cfg:         cfg,
		logger:      logger,
		ui:          NewStdPrinter(os.Stdout),
		Node:        n,
		stopLAN:     make(chan struct{}),
		Points:      pe,
		Quiz:        qe,
		Ledger:      ld,
		encChannels: make(map[string]channel.ChannelKey),
		otherPoints: make(map[string]proto.PointsSnapshot),
	}, nil
}

func (a *App) Start() error {
	if err := a.Node.Start(); err != nil {
		return err
	}

	lanCfg := discovery.DefaultLANConfig()

	if err := discovery.StartLANResponder(a.stopLAN, lanCfg, string(a.Node.ListenAddr()), a.Node.Name()); err != nil {
		a.logf("LAN responder failed: %v", err)
	}

	ps := discovery.NewPeerStore(discovery.DefaultPeerStorePath())
	mgr := discovery.NewManager(ps, lanCfg)
	mgr.Run(a.Node)

	// Broadcast initial signed points snapshot
	if snap, err := a.Points.SnapshotSelf(); err == nil {
		a.broadcastPoints(snap)
	}

	return nil
}

func (a *App) Run(ctx context.Context) error {
	PrintBanner(a.ui, a.Node)

	// CLI input loop
	go a.readStdin(ctx)

	// Event emitter
	go func() {
		for ev := range a.Node.Events() {
			switch ev.Type {
			case p2p.EventPeerConnected:
				a.ui.Printf("[NET] peer connected: %s (%s)\n", ev.PeerName, ev.PeerAddr)
			case p2p.EventPeerDisconnected:
				a.ui.Printf("[NET] peer disconnected: %s\n", ev.PeerID)
			}
		}
	}()

	// Inbound loop (envelopes)
	for {
		select {
		case <-ctx.Done():
			return nil
		case env, ok := <-a.Node.Incoming():
			if !ok {
				return nil
			}
			a.handleEnvelope(env)
		}
	}
}

func (a *App) StopAll() {
	select {
	case <-a.stopLAN:
		// already closed by someone
	default:
		close(a.stopLAN)
	}
	a.Node.Stop()
}

func (a *App) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
		return
	}
	log.Printf(format+"\n", args...)
}

func (a *App) broadcastPoints(snap proto.SignedPointsSnapshot) {
	body, _ := json.Marshal(snap)
	a.Node.Broadcast(proto.Gossip{ID: p2p.NewMsgID(), Channel: "points", Body: body})
}

// Helper used by /me
func (a *App) userIDHex() string {
	id := a.Node.Identity()
	return hex.EncodeToString(id.SignPub)
}

// For “settle” waits after quit
func sleepBrief() { time.Sleep(100 * time.Millisecond) }
