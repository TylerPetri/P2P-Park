package appdata

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	once    sync.Once
	dataDir string
)

// Dir returns the directory where P2P-Park stores its local state.
//
// Precedence:
//  1. P2P_PARK_DATA_DIR env var (absolute or relative)
//  2. If running via `go run` (temp go-build path), use the current working directory
//  3. Directory of the executable
//
// The returned directory is created if it does not exist.
func Dir() string {
	once.Do(func() {
		if v := strings.TrimSpace(os.Getenv("P2P_PARK_DATA_DIR")); v != "" {
			dataDir = filepath.Clean(v)
			_ = os.MkdirAll(dataDir, 0o700)
			return
		}

		exe, err := os.Executable()
		if err != nil {
			dataDir = defaultCWDDataDir()
			return
		}
		exe = filepath.Clean(exe)

		base := filepath.Dir(exe)
		if looksLikeGoRunTempBinary(exe) {
			base = mustGetwd()
		}

		dataDir = filepath.Join(base, ".p2p-park")
		_ = os.MkdirAll(dataDir, 0o700)
	})
	return dataDir
}

// Path returns an absolute path to a file inside Dir(), ensuring the directory exists.
func Path(filename string) string {
	d := Dir()
	p := filepath.Join(d, filepath.Clean(filename))
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	return p
}

func defaultCWDDataDir() string {
	base := mustGetwd()
	d := filepath.Join(base, ".p2p-park")
	_ = os.MkdirAll(d, 0o700)
	return d
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func looksLikeGoRunTempBinary(exe string) bool {
	lower := strings.ToLower(exe)
	if strings.Contains(lower, string(filepath.Separator)+"go-build") {
		return true
	}
	if runtime.GOOS == "windows" {
		return strings.Contains(lower, "\\go-build")
	}
	return false
}
