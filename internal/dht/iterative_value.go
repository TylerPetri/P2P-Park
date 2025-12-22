package dht

import (
	"context"
	"net"
	"sort"
	"time"

	"p2p-park/internal/proto"
)

type ValueLookupConfig struct {
	Alpha      int
	K          int
	RPCTimeout time.Duration
	MaxRounds  int
}

func DefaultValueLookupConfig() ValueLookupConfig {
	return ValueLookupConfig{
		Alpha:      3,
		K:          20,
		RPCTimeout: 1200 * time.Millisecond,
		MaxRounds:  32,
	}
}

func (d *DHT) IterativeFindValue(ctx context.Context, n Sender, key [32]byte, cfg ValueLookupConfig) (*proto.DHTRecord, bool, error) {
	if rec, ok := d.rs.Get(key, time.Now()); ok {
		ok = true
		return rec, true, nil
	}

	if cfg.Alpha <= 0 {
		cfg.Alpha = 3
	}
	if cfg.K <= 0 {
		cfg.K = 20
	}
	if cfg.RPCTimeout <= 0 {
		cfg.RPCTimeout = 1200 * time.Millisecond
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 32
	}

	target := NodeID(key)
	keyHex := KeyHex(key)

	start := time.Now()
	queries := 0
	ok := false
	defer func() { d.metrics.ObserveLookup("FIND_VALUE", queries, time.Since(start), ok) }()

	const (
		stUnqueried = iota
		stQuerying
		stDone
		stFailed
	)

	type cand struct {
		node  proto.DHTNode
		id    NodeID
		dist  NodeID
		state int
	}

	seen := map[string]*cand{} // keyed by NodeID hex

	seed := d.rt.Closest(target, cfg.K)
	for _, ni := range seed {
		c := &cand{
			node:  proto.DHTNode{NodeID: ni.NodeIDHex, PeerID: ni.PeerID, Addr: ni.Addr, Name: ni.Name},
			id:    ni.NodeID,
			dist:  Distance(ni.NodeID, target),
			state: stUnqueried,
		}
		seen[c.node.NodeID] = c
	}

	sorted := func() []*cand {
		out := make([]*cand, 0, len(seen))
		for _, c := range seen {
			out = append(out, c)
		}
		sort.Slice(out, func(i, j int) bool {
			return DistanceLess(out[i].dist, out[j].dist)
		})
		return out
	}

	for round := 0; round < cfg.MaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}

		cands := sorted()
		if len(cands) == 0 {
			return nil, false, nil
		}

		limit := len(cands)
		if limit > cfg.K*2 {
			limit = cfg.K * 2
		}
		toQuery := make([]*cand, 0, cfg.Alpha)
		for i := 0; i < limit && len(toQuery) < cfg.Alpha; i++ {
			if cands[i].state == stUnqueried {
				cands[i].state = stQuerying
				toQuery = append(toQuery, cands[i])
			}
		}

		if len(toQuery) == 0 {
			limit2 := len(cands)
			if limit2 > cfg.K {
				limit2 = cfg.K
			}
			left := false
			for i := 0; i < limit2; i++ {
				if cands[i].state == stUnqueried {
					left = true
					break
				}
			}
			if !left {
				return nil, false, nil
			}
			continue
		}

		type result struct {
			c   *cand
			w   proto.DHTWire
			err error
		}
		queries += len(toQuery)
		resCh := make(chan result, len(toQuery))

		for _, c := range toQuery {
			peerID := c.node.PeerID
			go func(c *cand, pid string) {
				resp, err := d.QueryFindValue(n, pid, keyHex, cfg.RPCTimeout)
				resCh <- result{c: c, w: resp, err: err}
			}(c, peerID)
		}

		for i := 0; i < len(toQuery); i++ {
			r := <-resCh
			if r.err != nil || r.w.Kind != "VALUE" {
				r.c.state = stFailed
				continue
			}
			r.c.state = stDone

			if r.w.Record != nil {
				if err := d.ValidateRecordAgainstKey(key, r.w.Record); err == nil {
					_ = d.rs.Put(key, r.w.Record, time.Now())
					ok = true
					return r.w.Record, true, nil
				}
				r.c.state = stFailed
				continue
			}

			nodes := r.w.Nodes
			if len(nodes) > cfg.K*2 {
				nodes = nodes[:cfg.K*2]
			}
			for _, nd := range nodes {
				if nd.NodeID == "" || nd.PeerID == "" || nd.Addr == "" {
					continue
				}
				if !NodeIDMatchesPeerID(nd.NodeID, nd.PeerID) {
					continue
				}
				if _, _, err := net.SplitHostPort(nd.Addr); err != nil {
					continue
				}
				if _, ok := seen[nd.NodeID]; ok {
					continue
				}
				id, err := ParseNodeIDHex(nd.NodeID)
				if err != nil {
					continue
				}

				d.rt.UpsertWithEviction(id, nd.PeerID, nd.Addr, nd.Name, func(tail NodeInfo) bool {
					resp, err := d.QueryPing(n, tail.PeerID, 800*time.Millisecond)
					return err == nil && resp.Kind == "PONG"
				})

				seen[nd.NodeID] = &cand{node: nd, id: id, dist: Distance(id, target), state: stUnqueried}
			}
		}

		cands = sorted()
		if len(cands) > cfg.K*8 {
			cands = cands[:cfg.K*8]
			keep := map[string]*cand{}
			for _, c := range cands {
				keep[c.node.NodeID] = c
			}
			seen = keep
		}
	}

	return nil, false, nil
}
