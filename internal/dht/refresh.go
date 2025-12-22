package dht

import (
	"context"
	"time"
)

// RunBucketRefresh periodically refreshes the routing table by performing lookups
// for random IDs in bucket ranges. Minimal version: just refreshes random targets.
func (d *DHT) RunBucketRefresh(ctx context.Context, n Sender, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	cfg := DefaultLookupConfig()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Minimal refresh: lookup a random ID (later: per-bucket ranges)
			target := RandomNodeID()
			_, _ = d.IterativeFindNode(n, target.Hex(), cfg)
		}
	}
}
