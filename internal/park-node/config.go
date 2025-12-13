package parknode

import "p2p-park/internal/netx"

type Config struct {
	Name       string
	Bind       string
	IsSeed     bool
	Bootstraps []netx.Addr
	Debug      bool
}
