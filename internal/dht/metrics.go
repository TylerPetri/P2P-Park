package dht

import "time"

// Metrics is intentionally tiny and dependency-free.
// Implementations must be thread-safe.
type Metrics interface {
	IncRPC(kind string, ok bool)
	ObserveLookup(kind string, queries int, duration time.Duration, ok bool)
	SetRoutingTableSize(n int)
	SetBucketOccupancy(bucket int, n int)
}

// NoopMetrics is the default.
type NoopMetrics struct{}

func (NoopMetrics) IncRPC(kind string, ok bool) {}
func (NoopMetrics) ObserveLookup(kind string, queries int, duration time.Duration, ok bool) {}
func (NoopMetrics) SetRoutingTableSize(n int) {}
func (NoopMetrics) SetBucketOccupancy(bucket int, n int) {}
