package grantsbolt

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"

	"p2p-park/internal/app/grants"
	"p2p-park/internal/proto"
)

const (
	bMeta  = "meta"
	bByID  = "grants_by_id"
	bByTS  = "grants_by_ts"
	kMaxTS = "max_ts"

	defaultTO = 2 * time.Second
)

// Store is a BoltDB-backed implementation of grants.Store.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) a BoltDB database at path.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("empty db path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: defaultTO})
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(bMeta)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bByID)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(bByTS)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Put(g proto.QuizGrant) (bool, error) {
	// Caller must verify. Still guard against obviously bad inputs to protect the DB.
	if g.GrantID == "" {
		return false, errors.New("missing grant id")
	}

	val, err := json.Marshal(g)
	if err != nil {
		return false, err
	}

	var inserted bool
	err = s.db.Update(func(tx *bolt.Tx) error {
		byID := tx.Bucket([]byte(bByID))
		byTS := tx.Bucket([]byte(bByTS))
		meta := tx.Bucket([]byte(bMeta))

		if byID.Get([]byte(g.GrantID)) != nil {
			return nil
		}

		if err := byID.Put([]byte(g.GrantID), val); err != nil {
			return err
		}
		if err := byTS.Put(tsKey(g.Timestamp, g.GrantID), nil); err != nil {
			return err
		}

		// update max_ts
		cur := decodeI64(meta.Get([]byte(kMaxTS)))
		if g.Timestamp > cur {
			if err := meta.Put([]byte(kMaxTS), encodeI64(g.Timestamp)); err != nil {
				return err
			}
		}

		inserted = true
		return nil
	})
	return inserted, err
}

func (s *Store) MaxTimestamp() (int64, error) {
	var out int64
	err := s.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(bMeta))
		out = decodeI64(meta.Get([]byte(kMaxTS)))
		return nil
	})
	return out, err
}

func (s *Store) RecentGrantIDs(n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	out := make([]string, 0, n)
	err := s.db.View(func(tx *bolt.Tx) error {
		byTS := tx.Bucket([]byte(bByTS))
		byID := tx.Bucket([]byte(bByID))
		c := byTS.Cursor()
		for k, _ := c.Last(); k != nil && len(out) < n; k, _ = c.Prev() {
			_, id := splitTSKey(k)
			if id == "" {
				continue
			}
			// ensure it still exists (defensive)
			if byID.Get([]byte(id)) != nil {
				out = append(out, id)
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) ListSince(since int64, limit int) ([]proto.QuizGrant, error) {
	if limit <= 0 {
		limit = 500
	}
	out := make([]proto.QuizGrant, 0, min(limit, 256))

	err := s.db.View(func(tx *bolt.Tx) error {
		byTS := tx.Bucket([]byte(bByTS))
		byID := tx.Bucket([]byte(bByID))

		c := byTS.Cursor()
		seek := tsKey(since, "")
		for k, _ := c.Seek(seek); k != nil && len(out) < limit; k, _ = c.Next() {
			_, id := splitTSKey(k)
			if id == "" {
				continue
			}
			raw := byID.Get([]byte(id))
			if raw == nil {
				continue
			}
			var g proto.QuizGrant
			if err := json.Unmarshal(raw, &g); err != nil {
				// Corruption: keep going, don't brick sync.
				continue
			}
			out = append(out, g)
		}
		return nil
	})
	return out, err
}

func (s *Store) LoadAll(fn func(g proto.QuizGrant) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		byTS := tx.Bucket([]byte(bByTS))
		byID := tx.Bucket([]byte(bByID))
		c := byTS.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			_, id := splitTSKey(k)
			if id == "" {
				continue
			}
			raw := byID.Get([]byte(id))
			if raw == nil {
				continue
			}
			var g proto.QuizGrant
			if err := json.Unmarshal(raw, &g); err != nil {
				continue
			}
			if err := fn(g); err != nil {
				return err
			}
		}
		return nil
	})
}

func tsKey(ts int64, grantID string) []byte {
	// big-endian timestamp for correct ordering; append 0x00 + id so Seek works.
	b := make([]byte, 8+1+len(grantID))
	binary.BigEndian.PutUint64(b[:8], uint64(ts))
	b[8] = 0
	copy(b[9:], grantID)
	return b
}

func splitTSKey(k []byte) (int64, string) {
	if len(k) < 9 {
		return 0, ""
	}
	ts := int64(binary.BigEndian.Uint64(k[:8]))
	id := string(k[9:])
	return ts, id
}

func encodeI64(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func decodeI64(b []byte) int64 {
	if len(b) != 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b))
}

// Compile-time check that Store satisfies the interface.
var _ grants.Store = (*Store)(nil)
