package grants

import "p2p-park/internal/proto"

// Store persists quiz grants and supports efficient "since timestamp" range queries.
type Store interface {
	// Close releases underlying resources.
	Close() error

	// Put stores a verified grant if it doesn't already exist.
	// Returns true if it was newly inserted.
	Put(g proto.QuizGrant) (bool, error)

	// MaxTimestamp returns the largest grant timestamp observed (0 if none).
	MaxTimestamp() (int64, error)

	// RecentGrantIDs returns up to n most recent GrantIDs (newest-first).
	RecentGrantIDs(n int) ([]string, error)

	// ListSince returns up to limit grants with Timestamp >= since, ordered by Timestamp asc.
	ListSince(since int64, limit int) ([]proto.QuizGrant, error)

	// LoadAll streams all grants in timestamp order.
	LoadAll(fn func(g proto.QuizGrant) error) error
}
