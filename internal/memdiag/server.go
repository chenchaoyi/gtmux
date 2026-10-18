// Package memdiag provides opt-in, local-only Go memory observations. It never
// installs handlers on the application HTTP server or changes GC settings.
package memdiag

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Server owns one private socket, not the caller's directory or exported files.
type Server struct {
	Path string
	dir  string
	file os.FileInfo
	http *http.Server
	once sync.Once
	err  error
}

// Start does nothing when dir is empty. Otherwise dir must already be an absolute,
// private directory owned by this user. Each run gets a new 0700 subdirectory;
// a stale socket can never be mistaken for this run or removed to make room.
func Start(dir, version string) (*Server, error) {
	if dir == "" {
		return nil, nil
	}
	if !filepath.IsAbs(dir) {
		return nil, errors.New("memory diagnostics require an absolute private directory")
	}
	if err := state.CheckPrivateDirectory(dir); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if info.Mode().Perm() != 0o700 {
		return nil, errors.New("memory diagnostics directory must have mode 0700")
	}
	// Resolve ancestor links (not the already rejected leaf link), then reject
	// ancestors another local user could replace. Sticky /tmp is safe for our
	// owned leaf; a non-sticky shared writable parent is not.
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	for parent := filepath.Dir(dir); ; parent = filepath.Dir(parent) {
		p, e := os.Stat(parent)
		if e != nil {
			return nil, e
		}
		if !protectedParent(p) {
			return nil, errors.New("memory diagnostics require protected parent directories")
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
	run, err := os.MkdirTemp(dir, fmt.Sprintf("mem-%d-", os.Getpid()))
	if err != nil {
		return nil, err
	}
	path := filepath.Join(run, "memory.sock")
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		_ = os.Remove(run)
		return nil, err
	}
	l.SetUnlinkOnClose(false)
	if err = os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		_ = os.Remove(path)
		_ = os.Remove(run)
		return nil, err
	}
	info, err = os.Lstat(path)
	if err != nil {
		_ = l.Close()
		_ = os.Remove(path)
		_ = os.Remove(run)
		return nil, err
	}
	s := &Server{Path: path, dir: run, file: info}
	s.http = &http.Server{Handler: newHandler(version), ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 4096}
	go func() { _ = s.http.Serve(l) }()
	return s, nil
}

func protectedParent(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	// The owner can replace children even in a 0755 or sticky directory. A
	// foreign owner's directory therefore cannot protect our socket subtree.
	return ok && (owner.Uid == 0 || int(owner.Uid) == os.Geteuid()) &&
		(info.Mode().Perm()&0o022 == 0 || info.Mode()&os.ModeSticky != 0)
}

// Close stops accepting reads and removes only this instance's socket/directory.
// A file substituted at the socket path is preserved. SIGKILL cannot run cleanup;
// the next start uses a different directory instead of trusting a stale socket.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.err = s.http.Close()
		info, err := os.Lstat(s.Path)
		if err == nil && os.SameFile(s.file, info) {
			s.err = errors.Join(s.err, os.Remove(s.Path), os.Remove(s.dir))
		} else if !os.IsNotExist(err) {
			s.err = errors.Join(s.err, errors.New("diagnostic socket changed; preserving directory"))
		}
	})
	return s.err
}
