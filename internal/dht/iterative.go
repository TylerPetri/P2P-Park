package dht

import (
	"sort"
	"time"

	"p2p-park/internal/proto"
)

type LookupConfig struct {
	Alpha      int
	K          int
	RPCTimeout time.Duration
	MaxRounds  int
}

func DefaultLookupConfig() LookupConfig {
	return LookupConfig{
		Alpha:      3,
		K:          20,
		RPCTimeout: 1200 * time.Millisecond,
		MaxRounds:  32,
	}
}

func (d *DHT) IterativeFindNode(n Sender, targetHex string, cfg LookupConfig) ([]proto.DHTNode, error) {
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

	target, err := ParseNodeIDHex(targetHex)
	if err != nil {
		return nil, err
	}

	type cand struct {
		node proto.DHTNode
		dist NodeID
	}

	// seed from routing table
	seed := d.rt.Closest(target, cfg.K)
	best := make([]cand, 0, cfg.K)
	for _, ni := range seed {
		best = append(best, cand{
			node: proto.DHTNode{ID: ni.IDHex, Addr: ni.Addr, Name: ni.Name},
			dist: Xor(ni.ID, target),
		})
	}

	queried := make(map[string]bool) // by peerID / nodeID hex
	seen := make(map[string]bool)
	for _, c := range best {
		seen[c.node.ID] = true
	}

	sort.Slice(best, func(i, j int) bool { return best[i].dist.Hex() < best[j].dist.Hex() })

	// helper to choose next α
	pickNext := func() []proto.DHTNode {
		out := make([]proto.DHTNode, 0, cfg.Alpha)
		for _, c := range best {
			if len(out) == cfg.Alpha {
				break
			}
			if queried[c.node.ID] {
				continue
			}
			queried[c.node.ID] = true
			out = append(out, c.node)
		}
		return out
	}

	closerFound := true
	rounds := 0

	for closerFound && rounds < cfg.MaxRounds {
		rounds++
		closerFound = false

		toQuery := pickNext()
		if len(toQuery) == 0 {
			break
		}

		type result struct {
			resp proto.DHTWire
			ok   bool
		}
		resCh := make(chan result, len(toQuery))

		for _, peer := range toQuery {
			peerID := peer.ID // IMPORTANT: in your system, peerID == nodeID hex (Identity.ID)
			go func(pid string) {
				resp, err := d.QueryFindNode(n, pid, targetHex, cfg.RPCTimeout)
				if err != nil {
					resCh <- result{ok: false}
					return
				}
				resCh <- result{resp: resp, ok: true}
			}(peerID)
		}

		for i := 0; i < len(toQuery); i++ {
			r := <-resCh
			if !r.ok || r.resp.Kind != "NODES" {
				continue
			}
			for _, nd := range r.resp.Nodes {
				if nd.ID == "" || seen[nd.ID] {
					continue
				}
				seen[nd.ID] = true

				id, err := ParseNodeIDHex(nd.ID)
				if err != nil {
					continue
				}
				d.rt.Upsert(id, nd.Addr, nd.Name)

				best = append(best, cand{node: nd, dist: Xor(id, target)})
				closerFound = true
			}
		}

		sort.Slice(best, func(i, j int) bool { return best[i].dist.Hex() < best[j].dist.Hex() })
		if len(best) > cfg.K {
			best = best[:cfg.K]
		}
	}

	out := make([]proto.DHTNode, 0, len(best))
	for _, c := range best {
		out = append(out, c.node)
	}
	return out, nil
}
