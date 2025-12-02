package discovery

import (
	"fmt"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
)

type Manager struct {
	Store       *PeerStore
	LANConfig   LANConfig
	Seeds       []string
	Interval    time.Duration
	MaxFailures int
}

// NewManager constructs a DiscoveryManager with sane defaults.
func NewManager(store *PeerStore, lanCfg LANConfig) *Manager {
	return &Manager{
		Store:       store,
		LANConfig:   lanCfg,
		Seeds:       DefaultSeeds,
		Interval:    30 * time.Second,
		MaxFailures: 5,
	}
}

// Run starts the periodic discovery loop.
func (m *Manager) Run(n *p2p.Node) {
	go func() {
		time.Sleep(500 * time.Millisecond)

		for {
			// 1) LAN
			listenAddr := n.ListenAddr()
			nc := p2p.NodeConfig{}
			name := nc.Name
			addrs, err := DiscoverLANPeers(m.LANConfig, string(listenAddr), name)
			if err != nil {
				fmt.Printf("[DISCOVERY] LAN error: %v\n", err)
			}

			if len(addrs) == 0 {
				fmt.Println("[DISCOVERY] no peers found on LAN")
			} else {
				fmt.Printf("[DISCOVERY] found %d peers: %v\n", len(addrs), addrs)
			}
			// 2) PeerStore
			addrs = append(addrs, m.Store.Candidates(m.MaxFailures)...)
			// 3) Seeds
			addrs = append(addrs, m.Seeds...)

			seen := make(map[string]struct{})
			for _, a := range addrs {
				if a == "" || a == string(listenAddr) {
					continue
				}
				if _, ok := seen[a]; ok {
					continue
				}
				seen[a] = struct{}{}

				go func(addr string) {
					if err := n.ConnectTo(netx.Addr(addr)); err != nil {
						fmt.Printf("[DISCOVERY] connect to %s failed: %v\n", addr, err)
						m.Store.NoteFailure(addr)
						return
					}
					m.Store.NoteSuccess(addr)
				}(a)
			}

			time.Sleep(m.Interval)
		}
	}()
}
