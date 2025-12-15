package dht

import (
	"fmt"
	"sync"

	"p2p-park/internal/proto"
)

type Sender interface {
	ID() string
	SendToPeer(id string, env proto.Envelope) error
	Logf(format string, args ...any)
}

// DHT is the package's primary engine.
// It owns routing, pending RPCs, and lookup behavior.
type DHT struct {
	selfIDHex string
	self      NodeID
	rt        *RoutingTable

	pendingMu sync.Mutex
	pending   map[string]chan proto.DHTWire

	store *Store
}

type Option func(*DHT)

func WithStore(path string) Option {
	return func(d *DHT) {
		d.store = NewStore(path)
	}
}

func New(selfIDHex string, opts ...Option) (*DHT, error) {
	self, err := ParseNodeIDHex(selfIDHex)
	if err != nil {
		return nil, fmt.Errorf("dht: invalid self id: %w", err)
	}

	d := &DHT{
		selfIDHex: selfIDHex,
		self:      self,
		rt:        NewRoutingTable(self, 20),
		pending:   make(map[string]chan proto.DHTWire),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

func (d *DHT) Routing() *RoutingTable { return d.rt }

func (d *DHT) OnPeerSeen(peerIDHex, addr, name string) {
	id, err := ParseNodeIDHex(peerIDHex)
	if err != nil {
		return
	}

	d.rt.Upsert(id, addr, name)

	if d.store != nil && addr != "" {
		d.store.NoteSuccess(peerIDHex, addr, name)
	}
}

func (d *DHT) BootstrapAddrs(limit int) []string {
	if d.store == nil {
		return nil
	}
	return d.store.Candidates(5, limit)
}
