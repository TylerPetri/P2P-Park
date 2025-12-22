package dht

import (
	"sync/atomic"
	"time"
)

type AtomicMetrics struct {
	rpcOK    atomic.Uint64
	rpcFail  atomic.Uint64
	lookups  atomic.Uint64
	lookupOK atomic.Uint64
	lookupFail atomic.Uint64
	lookupQueries atomic.Uint64
	rtSize atomic.Int64
}

func (m *AtomicMetrics) IncRPC(kind string, ok bool) {
	if ok { m.rpcOK.Add(1) } else { m.rpcFail.Add(1) }
}

func (m *AtomicMetrics) ObserveLookup(kind string, queries int, duration time.Duration, ok bool) {
	m.lookups.Add(1)
	m.lookupQueries.Add(uint64(queries))
	if ok { m.lookupOK.Add(1) } else { m.lookupFail.Add(1) }
}

func (m *AtomicMetrics) SetRoutingTableSize(n int) { m.rtSize.Store(int64(n)) }
func (m *AtomicMetrics) SetBucketOccupancy(bucket int, n int) {}

func (m *AtomicMetrics) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"rpc_ok": m.rpcOK.Load(),
		"rpc_fail": m.rpcFail.Load(),
		"lookups": m.lookups.Load(),
		"lookup_ok": m.lookupOK.Load(),
		"lookup_fail": m.lookupFail.Load(),
		"lookup_queries": m.lookupQueries.Load(),
	}
}
