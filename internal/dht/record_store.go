package dht

import (
	"sync"
	"time"

	"p2p-park/internal/proto"
)

type RecordStore interface {
	Get(key [32]byte, now time.Time) (*proto.DHTRecord, bool)
	Put(key [32]byte, rec *proto.DHTRecord, now time.Time) error
	Delete(key [32]byte) error
	SweepExpired(now time.Time) int
	ForEach(fn func(key [32]byte, rec *proto.DHTRecord) bool)
	Len() int
}

type MemRecordStore struct {
	mu   sync.RWMutex
	data map[[32]byte]*proto.DHTRecord
}

func NewMemRecordStore() *MemRecordStore {
	return &MemRecordStore{data: make(map[[32]byte]*proto.DHTRecord)}
}

func (m *MemRecordStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

func copyRecord(in *proto.DHTRecord) *proto.DHTRecord {
	if in == nil {
		return nil
	}
	out := *in
	if in.Value != nil {
		out.Value = append([]byte(nil), in.Value...)
	}
	if in.PubKey != nil {
		out.PubKey = append([]byte(nil), in.PubKey...)
	}
	if in.Sig != nil {
		out.Sig = append([]byte(nil), in.Sig...)
	}
	return &out
}

func (m *MemRecordStore) Get(key [32]byte, now time.Time) (*proto.DHTRecord, bool) {
	m.mu.RLock()
	rec := m.data[key]
	m.mu.RUnlock()
	if rec == nil {
		return nil, false
	}
	if rec.ExpiresUnix != 0 && now.Unix() > rec.ExpiresUnix {
		return nil, false
	}
	return copyRecord(rec), true
}

func (m *MemRecordStore) Put(key [32]byte, rec *proto.DHTRecord, now time.Time) error {
	if rec == nil {
		return ErrBadRecord
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Enforce mutable seq rule (if same key exists)
	if rec.Type == RecordMutable {
		old := m.data[key]
		if old != nil && old.Type == RecordMutable {
			if rec.Seq <= old.Seq {
				return ErrSeqTooLow
			}
		}
	}

	m.data[key] = copyRecord(rec)
	return nil
}

func (m *MemRecordStore) Delete(key [32]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemRecordStore) SweepExpired(now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	n := 0
	for k, rec := range m.data {
		if rec != nil && rec.ExpiresUnix != 0 && now.Unix() > rec.ExpiresUnix {
			delete(m.data, k)
			n++
		}
	}
	return n
}

func (m *MemRecordStore) ForEach(fn func(key [32]byte, rec *proto.DHTRecord) bool) {
	m.mu.RLock()
	// Snapshot keys so fn can be slow without holding lock too long.
	keys := make([][32]byte, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	m.mu.RUnlock()

	for _, k := range keys {
		m.mu.RLock()
		rec := m.data[k]
		m.mu.RUnlock()
		if rec == nil {
			continue
		}
		if !fn(k, copyRecord(rec)) {
			return
		}
	}
}
