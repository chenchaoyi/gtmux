// Package hqinput serializes gtmux's competing HQ input channels. It does not
// replace draft checks or claim that a human cannot type during an attempt.
package hqinput

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func Acquire() (release func(), acquired bool, err error) {
	return acquire(filepath.Join(state.Dir(), "hqwake", "input.lock"))
}
func acquire(path string) (release func(), acquired bool, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, true, nil
}
