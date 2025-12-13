package p2p

func (n *Node) Logf(format string, args ...any) {
	if !n.cfg.Debug {
		return
	}
	if n.cfg.Logger != nil {
		n.cfg.Logger.Printf("[node %s] "+format, append([]any{n.id.ID[:8]}, args...)...)
	}
}
