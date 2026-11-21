package hqnudge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
	"golang.org/x/sys/unix"
)

// The queue lock protects short on-disk operations only. The HQ input lock can
// span a driver call, so producers must stay able to enqueue while it is held.
func lockQueue() (func(), error) {
	if err := state.MkdirAll(queueDir(), 0700); err != nil {
		return nil, err
	}
	lockDir := filepath.Join(state.Dir(), "hqwake")
	if err := state.MkdirAll(lockDir, 0700); err != nil {
		return nil, err
	}
	file, err := state.OpenFile(filepath.Join(lockDir, "wake-queue.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() { _ = file.Close() }, nil
}

func pendingKey(field string) string {
	for _, entry := range readQueue() {
		if entry.Type().IsRegular() && queuedKey(entry.Name()) == field {
			return entry.Name()
		}
	}
	return ""
}

func queuedKey(path string) string {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, ".txt") {
		return ""
	}
	at := strings.Index(name, ".k-")
	if at < 0 {
		return ""
	}
	field, _, _ := strings.Cut(name[at+1:], ".")
	return "." + field
}

func takeQueued(src, dst string) bool {
	release, err := lockQueue()
	if err != nil {
		diag.ReportStoreFailure("hq-wake-queue", err)
		return false
	}
	defer release()
	if _, err := os.Lstat(dst); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	return os.Rename(src, dst) == nil
}
