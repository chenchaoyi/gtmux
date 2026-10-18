package notify

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const queueLimit = 256
const queueMaxAge = 30 * time.Second // matches the menu-bar consumer's stale-request rule

func queueLease() *state.Lease {
	deadline := time.Now().Add(25 * time.Millisecond)
	for {
		lease, err := state.TryLease("notify-queue")
		if err == nil {
			return lease
		}
		if err != state.ErrLeaseHeld || !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

// Only the timestamped files this producer owns are eligible. User logs, other
// state and unrecognized files are never part of notification queue maintenance.
func queueStamp(name string) (int64, bool) {
	base := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".tmp")
	if !strings.HasSuffix(base, ".json") {
		return 0, false
	}
	stamp, rest, ok := strings.Cut(base, "-")
	n, err := strconv.ParseInt(stamp, 10, 64)
	return n, ok && err == nil && n > 0 && len(rest) > len(".json")
}

func pruneQueue(dir string, now time.Time) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	type queued struct {
		name  string
		stamp int64
	}
	var pending []queued
	for {
		entries, readErr := f.ReadDir(128)
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			stamp, owned := queueStamp(entry.Name())
			if !owned {
				continue
			}
			if now.Sub(time.Unix(0, stamp)) > queueMaxAge {
				if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
					return false
				}
			} else if strings.HasSuffix(entry.Name(), ".json") {
				pending = append(pending, queued{entry.Name(), stamp})
			}
		}
		sort.Slice(pending, func(i, j int) bool { return pending[i].stamp < pending[j].stamp })
		for len(pending) >= queueLimit {
			if err := os.Remove(filepath.Join(dir, pending[0].name)); err != nil && !os.IsNotExist(err) {
				return false
			}
			pending = pending[1:]
		}
		if readErr == io.EOF {
			return true
		}
		if readErr != nil {
			return false
		}
	}
}
