package paths

import (
	"os"
	"path/filepath"
)

// DefaultDataDir returns a per-user directory appropriate for persisting node state.
// It prefers os.UserConfigDir and falls back to the current directory.
func DefaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "p2p-park")
	}
	return ".p2p-park"
}

// EnsureDir makes sure dir exists and returns the cleaned path.
func EnsureDir(dir string) (string, error) {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
