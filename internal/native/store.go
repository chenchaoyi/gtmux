package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// One stable lock inode avoids both unlink/waiter races and unbounded per-ID locks.
// All native writers, retirement and liveness pruning use this short bounded lock.
func withStoreLock(fn func() error) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(Dir(), ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("native store lock timeout: %w", err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
	return fn()
}

func atomicJSON(path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".native-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func removeFile(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// removeIfUnchanged also protects pruning after a slow process-liveness probe.
func removeIfUnchanged(expected Record) (bool, error) {
	removed := false
	err := withStoreLock(func() error {
		current, ok := Load(expected.SessionID)
		if !ok || current != expected {
			return nil
		}
		if err := removeFile(fileFor(expected.SessionID)); err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

func removeSnapshot(r Record) {
	if _, err := removeIfUnchanged(r); err != nil {
		diag.For("native").Warn("native.prune.failed", "could not prune native observation", "agent_session", r.SessionID, "revision", r.Revision, "error", err)
	}
}
