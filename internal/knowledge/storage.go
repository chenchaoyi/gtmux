package knowledge

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// One lock covers selection, settlement and source appends, including other CLI
// processes. The lock file is never renamed with either data file.
func withKnowledgeLock(run func() error) error {
	return WithStorageLock(func() error {
		if err := os.MkdirAll(Dir(), 0700); err != nil {
			return err
		}
		return run()
	})
}

func knowledgeStorageLock(run func() error) error {
	lockDir := filepath.Join(state.Dir(), "locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return err
	}
	home := state.HQHome()
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(home)); err == nil {
		home = filepath.Join(resolved, filepath.Base(home))
	}
	lockName := fmt.Sprintf("knowledge-%x.lock", sha256.Sum256([]byte(home)))
	f, err := os.OpenFile(filepath.Join(lockDir, lockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return run()
}

// atomicAppend keeps the exact previous bytes and appends one complete line. A
// failed write cannot leave a parseable settlement without its source evidence,
// or a partial line that swallows the following operation. The caller holds the
// knowledge lock. committed is true only after rename, including a later sync error.
func atomicAppend(path string, line []byte) (bool, error) {
	return atomicAppendLines(path, [][]byte{line})
}

func atomicAppendLines(path string, lines [][]byte) (committed bool, err error) {
	for _, line := range lines {
		if len(line) >= 1024*1024 {
			return false, fmt.Errorf("knowledge record exceeds the 1 MiB read limit")
		}
	}
	mode := os.FileMode(0o600)
	if info, e := os.Lstat(path); e == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o222 == 0 {
			return false, fmt.Errorf("knowledge file is not a writable regular file: %s", path)
		}
	} else if !os.IsNotExist(e) {
		return false, e
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".knowledge-write-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(mode); err != nil {
		return false, err
	}
	src, err := os.Open(path)
	if err == nil {
		_, copyErr := io.Copy(tmp, src)
		closeErr := src.Close()
		if copyErr != nil {
			return false, copyErr
		}
		if closeErr != nil {
			return false, closeErr
		}
		end, e := tmp.Seek(0, io.SeekCurrent)
		if e != nil {
			return false, e
		}
		if end > 0 {
			var last [1]byte
			if _, e := tmp.ReadAt(last[:], end-1); e != nil {
				return false, e
			}
			if last[0] != '\n' {
				if _, e := tmp.Write([]byte{'\n'}); e != nil {
					return false, e
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	for _, line := range lines {
		if n, e := tmp.Write(line); e != nil {
			return false, e
		} else if n != len(line) {
			return false, io.ErrShortWrite
		}
	}
	if err := tmp.Sync(); err != nil {
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return false, err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return true, err
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if syncErr != nil {
		return true, syncErr
	}
	return true, closeErr
}

// WithStorageLock lets HQ archive publication share the ledger writer's stable lock.
// The callback must not invoke another knowledge writer.
func WithStorageLock(run func() error) error { return knowledgeStorageLock(run) }
