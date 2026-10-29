//go:build darwin || linux

package codexbridge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/gorilla/websocket"
)

// Run launches an admitted interactive client. Admission happens before this
// function; no fallback or second launch is permitted after the child starts.
func Run(bin string, args []string, pane string) int {
	endpoint, listener, cleanup, err := frontendListener()
	if err != nil {
		return -1 // the caller can execute the original launch unchanged
	}
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var connections sync.WaitGroup
	var lifecycle sync.Mutex
	closing := false
	var sequence atomic.Uint64
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	type peerKey struct{}
	server.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		pid, ok := codexrpc.LocalPeerPID(c)
		if !ok {
			c.Close()
			return ctx
		}
		return context.WithValue(ctx, peerKey{}, pid)
	}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lifecycle.Lock()
		if closing {
			lifecycle.Unlock()
			http.Error(w, "closing", http.StatusServiceUnavailable)
			return
		}
		connections.Add(1)
		lifecycle.Unlock()
		defer connections.Done()
		peer, _ := r.Context().Value(peerKey{}).(int)
		instance, err := instanceID()
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		up, err := codexrpc.DialLocal(ctx)
		if err != nil {
			http.Error(w, "local daemon unavailable", http.StatusServiceUnavailable)
			return
		}
		defer up.Close()
		backendPID, verified := codexrpc.LocalPeerPID(up.UnderlyingConn())
		if !verified {
			http.Error(w, "unverified daemon", http.StatusForbidden)
			return
		}
		attachment, err := resume.BeginCodexFrontend(pane, instance, peer, backendPID, sequence.Add(1))
		if err != nil {
			http.Error(w, "unverified frontend", http.StatusForbidden)
			return
		}
		// This endpoint has a same-user, exact foreground Codex PID gate above.
		// It is a private Unix socket, never a browser or TCP authentication path.
		u := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		down, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer down.Close()
		down.SetReadLimit(64 << 20)
		var mu sync.Mutex
		defer func() { mu.Lock(); resume.EndCodexFrontend(attachment); mu.Unlock() }()
		o := &observer{home: attachment.HomeID, bind: func(sid, cwd string) {
			mu.Lock()
			defer mu.Unlock()
			if attachment.Sequence != sequence.Load() {
				return
			}
			bound, err := resume.BindCodexFrontend(attachment, sid, cwd)
			if err != nil {
				var receiptErr *resume.CodexBindingReceiptError
				if errors.As(err, &receiptErr) {
					attachment = bound
				}
				diag.For("codex-bridge").Warn("codex.frontend.binding.failed", "could not verify frontend attachment", "pane", pane, "error", err)
				return
			}
			attachment = bound
		}}
		ended := make(chan error, 2)
		go func() { ended <- forward(down, up, o.request) }()
		go func() { ended <- forward(up, down, o.response) }()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ended:
				down.Close()
				up.Close()
				<-ended
				return
			case <-ctx.Done():
				down.Close()
				up.Close()
				<-ended
				<-ended
				return
			case <-tick.C:
				mu.Lock()
				if attachment.SessionID != "" && attachment.Sequence == sequence.Load() {
					if fresh, err := resume.RefreshCodexFrontend(attachment); err == nil {
						attachment = fresh
					}
				}
				mu.Unlock()
			}
		}
	})
	go func() { _ = server.Serve(listener) }()
	defer func() {
		lifecycle.Lock()
		closing = true
		lifecycle.Unlock()
		_ = server.Close()
		cancel()
		connections.Wait()
	}()
	cwd, err := os.Getwd()
	if err != nil {
		return -1
	}
	argv := append([]string{"--remote", "unix://" + endpoint}, LocalArgs(args, cwd)...)
	cmd := exec.Command(bin, argv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "gtmux:", err)
		return 1
	}
	diag.Did("act.codex.launch", pane, diag.OK, "started local Codex terminal client", "mode", "frontend-bridge")
	// Terminal-generated signals already reach both processes in the foreground
	// group. Catching SIGINT prevents the bridge from dying before Codex handles it.
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		for {
			select {
			case s := <-signals:
				if s != syscall.SIGINT && s != syscall.SIGQUIT {
					_ = cmd.Process.Signal(s)
				}
			case <-finished:
				return
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) {
			return e.ExitCode()
		}
		return 1
	}
	return 0
}

func forward(from, to *websocket.Conn, observe func([]byte)) error {
	for {
		kind, body, err := from.ReadMessage()
		if err != nil {
			return err
		}
		if kind == websocket.TextMessage {
			observe(body)
		}
		if err := to.WriteMessage(kind, body); err != nil {
			return err
		}
	}
}

func instanceID() (string, error) {
	var id [16]byte
	_, err := rand.Read(id[:])
	return hex.EncodeToString(id[:]), err
}

// Match the official verified_local_daemon_socket layout so the TUI retains
// local-daemon semantics. Never replace the real daemon's rendezvous or socket.
func frontendListener() (string, net.Listener, func(), error) {
	root := filepath.Join(state.Dir(), "codex-frontends")
	if err := state.MkdirAll(root, 0700); err != nil {
		return "", nil, nil, err
	}
	dir, err := state.MkdirTemp(root, "client-")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	alias := filepath.Join(canonical, "frontend.sock")
	tmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	protectedDir := filepath.Join(tmp, "codex-daemon-"+strconv.Itoa(os.Geteuid()))
	info, err := os.Lstat(protectedDir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		cleanup()
		return "", nil, nil, errors.New("verified daemon socket directory unavailable")
	}
	physical := filepath.Join(protectedDir, fmt.Sprintf("%x", sha256.Sum256([]byte(alias))))
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: physical, Net: "unix"})
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	if err = os.Chmod(physical, 0600); err == nil {
		err = os.Symlink(physical, alias)
	}
	if err != nil {
		l.Close()
		cleanup()
		return "", nil, nil, err
	}
	return alias, l, func() { l.Close(); cleanup() }, nil
}
