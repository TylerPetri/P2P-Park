package sim_test

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"testing"

	"p2p-park/internal/dht"
	sim "p2p-park/internal/dht/sim"
)

func randPeerID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func TestSim_FindNode_StarBootstrap(t *testing.T) {
	nw := sim.NewNetwork(1)

	const N = 25
	nodes := make([]*sim.Node, 0, N)

	for i := 0; i < N; i++ {
		pid := randPeerID(t) // peerID = hex(pubkey)
		d, err := dht.New(pid, dht.WithDiversityPolicy(dht.DiversityPolicy{MaxPerSubnet: 0}))
		if err != nil {
			t.Fatalf("new dht: %v", err)
		}
		n := sim.NewNode(nw, pid, "127.0.0.1:0", "n", d)
		nodes = append(nodes, n)
	}

	// Star bootstrap: everyone knows node0, node0 knows everyone.
	for i := 1; i < N; i++ {
		nodes[i].DHT().ObservePeer(nodes[i], nodes[0].ID(), "127.0.0.1:0", "root")
		nodes[0].DHT().ObservePeer(nodes[0], nodes[i].ID(), "127.0.0.1:0", "leaf")
	}

	targetPeer := nodes[N-1].ID()
	targetNode, _ := dht.NodeIDFromPeerID(targetPeer)

	cfg := dht.DefaultLookupConfig()
	cfg.MaxRounds = 32 // allow convergence in sim

	resp, err := nodes[1].DHT().IterativeFindNode(nodes[1], targetNode.Hex(), cfg)
	if err != nil {
		t.Fatalf("iterfind: %v", err)
	}

	// Compute the true global k-closest peers to targetNode (excluding the querying node itself if you want).
	type pair struct {
		peerID string
		dist   dht.NodeID
	}
	all := make([]pair, 0, len(nodes))
	for _, n := range nodes {
		peer := n.ID()
		nid, err := dht.NodeIDFromPeerID(peer)
		if err != nil {
			t.Fatalf("bad peer id in sim: %v", err)
		}
		all = append(all, pair{
			peerID: peer,
			dist:   dht.Distance(nid, dht.NodeID(targetNode)),
		})
	}

	sort.Slice(all, func(i, j int) bool {
		return dht.DistanceLess(all[i].dist, all[j].dist)
	})

	k := cfg.K
	if k <= 0 {
		k = 20
	}
	if k > len(all) {
		k = len(all)
	}
	want := make(map[string]bool, k)
	for i := 0; i < k; i++ {
		want[all[i].peerID] = true
	}

	got := make(map[string]bool, len(resp))
	for _, nd := range resp {
		got[nd.PeerID] = true
	}

	// Require that we got all globally closest-k peers (or at least most of them if your RT may omit self).
	for peer := range want {
		if !got[peer] {
			t.Fatalf("expected response to include globally closest peer %s (k=%d)", peer, k)
		}
	}
}