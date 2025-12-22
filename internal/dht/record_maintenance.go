package dht

import (
	"context"
	"time"
)

type MaintenanceConfig struct {
	SweepInterval     time.Duration
	RepublishInterval time.Duration
}

func DefaultMaintenanceConfig() MaintenanceConfig {
	return MaintenanceConfig{
		SweepInterval:     2 * time.Minute,
		RepublishInterval: 30 * time.Minute,
	}
}

func (d *DHT) RunRecordMaintenance(ctx context.Context, n Sender, cfg MaintenanceConfig) {
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = 2 * time.Minute
	}
	if cfg.RepublishInterval <= 0 {
		cfg.RepublishInterval = 30 * time.Minute
	}

	sweepT := time.NewTicker(cfg.SweepInterval)
	defer sweepT.Stop()

	repT := time.NewTicker(cfg.RepublishInterval)
	defer repT.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-sweepT.C:
			_ = d.rs.SweepExpired(time.Now())

		case <-repT.C:
			d.republishOwned(ctx, n)
		}
	}
}

func (d *DHT) republishOwned(ctx context.Context, n Sender) {
	now := time.Now()

	// Snapshot keys to avoid holding lock while doing network IO
	d.ownedMu.Lock()
	keys := make([][32]byte, 0, len(d.owned))
	for k, st := range d.owned {
		if st.nextRepublish.IsZero() || !now.Before(st.nextRepublish) {
			keys = append(keys, k)
		}
	}
	d.ownedMu.Unlock()

	for _, k := range keys {
		if err := ctx.Err(); err != nil {
			return
		}
		rec, ok := d.rs.Get(k, time.Now())
		if !ok || rec == nil {
			d.ownedMu.Lock()
			delete(d.owned, k)
			d.ownedMu.Unlock()
			continue
		}

		_ = d.PublishRecord(ctx, n, k, rec, DefaultPublishConfig())

		d.ownedMu.Lock()
		d.owned[k] = ownedRec{nextRepublish: time.Now().Add(30 * time.Minute)}
		d.ownedMu.Unlock()
	}
}
