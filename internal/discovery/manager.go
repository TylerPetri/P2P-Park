package discovery

import (
	"fmt"
	"time"

	"p2p-park/internal/netx"
	"p2p-park/internal/p2p"
)

type Manager struct {
	Strategies []Strategy
	PeerStore  *PeerStore
}

func NewManager(ps *PeerStore, strategies ...Strategy) *Manager {
	return &Manager{
		Strategies: strategies,
		PeerStore:  ps,
	}
}

func (m *Manager) Run(n *p2p.Node) {
	go func() {
		for {
			for _, s := range m.Strategies {
				addrs := s.Discover(n)
				for _, addr := range addrs {
					go func(addr string) {
						if err := n.ConnectTo(netx.Addr(addr)); err != nil {
							fmt.Printf("[DISCOVERY] connect to %s failed: %v\n", addr, err)
							if m.PeerStore != nil {
								m.PeerStore.NoteFailure(addr)
							}
							return
						}
						if m.PeerStore != nil {
							m.PeerStore.NoteSuccess(addr)
						}
					}(addr)
				}
			}
			time.Sleep(30 * time.Second)
		}
	}()
}
