package events

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/usercfg"
	"golang.org/x/sys/unix"
)

// A stopped or read-only journal cannot attest to its own missed writes. Pin
// the independently stored recording configuration at the authority source.
// If config.json is absent, its closest existing parent witnesses creation
// and removal. A changed witness requires fresh authority, even if the JSON
// has returned to the same value.
func recordingPolicy() (string, int64) {
	path := usercfg.Path()
	for missing := 0; ; missing++ {
		var st unix.Stat_t
		err := unix.Lstat(path, &st)
		if errors.Is(err, os.ErrNotExist) && path != filepath.Dir(path) {
			path = filepath.Dir(path)
			continue
		}
		mode := st.Mode & unix.S_IFMT
		if err != nil || st.Ctim.Sec <= 0 || missing == 0 && mode != unix.S_IFREG || missing > 0 && mode != unix.S_IFDIR {
			return "", 0
		}
		key := fmt.Sprintf("%d/%d/%d/%d/%d", missing, st.Dev, st.Ino, st.Ctim.Sec, st.Ctim.Nsec)
		return fmt.Sprintf("%x", sha256.Sum256([]byte(key))), int64(st.Ctim.Sec)
	}
}

func RecordingPolicy() string {
	key, _ := recordingPolicy()
	if key == "" {
		return "unknown"
	}
	return key
}

// Legacy sources carry only seconds. Accept their policy history only when
// this independent witness strictly predates the original source.
func RecordingPolicyCovers(key string, at int64) bool {
	current, changed := recordingPolicy()
	if current == "" {
		return false
	}
	if key == "" {
		return changed < at
	}
	return key == current
}
