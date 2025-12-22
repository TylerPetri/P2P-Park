package dht

import (
	"fmt"
	"sync"
	"time"

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
	rs    RecordStore

	ownedMu sync.Mutex
	owned   map[[32]byte]ownedRec

	rlMu sync.Mutex
	rl   map[string]*tokenBucket

	inflightMu sync.Mutex
	inflight   map[string]int

	metrics Metrics
}

type ownedRec struct {
	nextRepublish time.Time
}

type Option func(*DHT)

func WithStore(path string) Option {
	return func(d *DHT) {
		d.store = NewStore(path)
	}
}

func WithRecordStore(rs RecordStore) Option {
	return func(d *DHT) { d.rs = rs }
}

func New(selfIDHex string, opts ...Option) (*DHT, error) {
	selfNode, err := NodeIDFromPeerID(selfIDHex)
	if err != nil {
		return nil, fmt.Errorf("dht: invalid self peer id: %w", err)
	}

	d := &DHT{
		selfIDHex: selfIDHex,
		self:      selfNode,
		rt:        NewRoutingTable(selfNode, 20),
		rs:        NewMemRecordStore(),
		pending:   make(map[string]chan proto.DHTWire),
		owned:     make(map[[32]byte]ownedRec),
		rl:        make(map[string]*tokenBucket),
		inflight:  make(map[string]int),
		metrics:   NoopMetrics{},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

func (d *DHT) Routing() *RoutingTable { return d.rt }

func (d *DHT) ObservePeer(n Sender, peerID, addr, name string) {
	nodeID, err := NodeIDFromPeerID(peerID)
	if err != nil {
		return
	}

	d.rt.UpsertWithEviction(nodeID, peerID, addr, name, func(tail NodeInfo) bool {
		resp, err := d.QueryPing(n, tail.PeerID, 800*time.Millisecond)
		return err == nil && resp.Kind == "PONG"
	})

	bi := BucketIndex(d.self, nodeID)
	d.metrics.SetBucketOccupancy(bi, d.rt.BucketSize(bi))
	d.metrics.SetRoutingTableSize(d.rt.Size())

	if d.store != nil && addr != "" {
		d.store.NoteSuccess(peerID, addr, name)
	}
}

func (d *DHT) OnPeerSeen(peerIDHex, addr, name string) {
	nodeID, err := NodeIDFromPeerID(peerIDHex)
	if err != nil {
		return
	}
	d.rt.Upsert(nodeID, peerIDHex, addr, name)

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

func WithMetrics(m Metrics) Option {
	return func(d *DHT) {
		if m == nil {
			d.metrics = NoopMetrics{}
			return
		}
		d.metrics = m
	}
}

func WithDiversityLimit(maxPerSubnet int) Option {
	return func(d *DHT) {
		d.rt.SetDiversityLimit(maxPerSubnet)
	}
}
