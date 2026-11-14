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
	var sequence connectionEpoch
	launch := newLaunchGate()
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
		started := time.Now()
		admitCtx, admitCancel := context.WithTimeout(r.Context(), admissionBudget)
		defer admitCancel()
		refuse := func(err error, attempts, status int) {
			reason := admissionReason(err)
			diag.For("codex-bridge").Warn("codex.frontend.startup.refused", "local frontend startup admission failed",
				"pane", pane, "reason", reason, "attempts", attempts, "elapsed_ms", time.Since(started).Milliseconds())
			fmt.Fprintf(os.Stderr, "gtmux: local Codex startup refused (%s); inspect gtmux logs --since 10m --event 'codex.frontend.startup.*'\n", reason)
			http.Error(w, "local frontend admission: "+reason, status)
		}
		childPID, err := launch.wait(admitCtx)
		if err != nil {
			refuse(err, 0, http.StatusServiceUnavailable)
			return
		}
		instance, err := instanceID()
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		up, err := dialForAdmission(admitCtx, codexrpc.DialLocal)
		if err != nil {
			refuse(err, 0, http.StatusServiceUnavailable)
			return
		}
		defer up.Close()
		backendPID, verified := codexrpc.LocalPeerPID(up.UnderlyingConn())
		if !verified {
			refuse(&resume.FrontendAdmissionError{Code: "daemon-peer-unverified"}, 0, http.StatusForbidden)
			return
		}
		connectionSequence := sequence.reserve()
		attachment, attempts, err := awaitFrontend(admitCtx, func(c context.Context) (resume.CodexFrontend, error) {
			return resume.BeginCodexFrontendFromLaunch(c, pane, instance, peer, backendPID, connectionSequence, childPID)
		}, admissionPause)
		if err != nil {
			refuse(err, attempts, admissionStatus(err))
			return
		}
		// The admission deadline bounds readiness only, not the lifetime of the
		// accepted interactive transport.
		if err := up.UnderlyingConn().SetDeadline(time.Time{}); err != nil {
			refuse(&resume.FrontendAdmissionError{Code: "daemon-transport-unavailable"}, attempts, http.StatusServiceUnavailable)
			return
		}
		// This endpoint has a same-user, exact foreground Codex PID gate above.
		// It is a private Unix socket, never a browser or TCP authentication path.
		upgradeTimeout, err := admissionUpgradeTimeout(admitCtx)
		if err != nil {
			refuse(err, attempts, http.StatusServiceUnavailable)
			return
		}
		u := websocket.Upgrader{HandshakeTimeout: upgradeTimeout, CheckOrigin: func(*http.Request) bool { return true }}
		down, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer down.Close()
		active, err := completeAdmission(admitCtx, &sequence, connectionSequence)
		if err != nil {
			diag.For("codex-bridge").Warn("codex.frontend.startup.refused", "frontend upgrade admission did not complete",
				"pane", pane, "reason", admissionReason(err), "attempts", attempts, "elapsed_ms", time.Since(started).Milliseconds())
			return
		}
		if !active {
			return
		}
		diag.For("codex-bridge").Info("codex.frontend.startup.ready", "verified local frontend connection",
			"pane", pane, "attempts", attempts, "elapsed_ms", time.Since(started).Milliseconds())
		down.SetReadLimit(64 << 20)
		binding := newConnectionBinding(ctx, attachment, &sequence, func(kind string, err error, count int) {
			if kind == "relocated" {
				diag.For("codex-bridge").Info("codex.frontend.location.updated", "verified frontend followed its current pane location", "pane", pane)
				return
			}
			if kind == "ready" {
				diag.For("codex-bridge").Info("codex.frontend.binding.ready", "verified frontend binding recovered", "pane", pane, "attempts", count)
				return
			}
			diag.For("codex-bridge").Warn("codex.frontend.binding.failed", "could not verify frontend attachment", "pane", pane, "error", err, "attempts", count, "stage", kind)
		})
		defer binding.close()
		o := &observer{home: attachment.HomeID, bind: binding.stage, selectRoot: binding.selectRoot, resetHome: binding.resetHome}

		trust := newTrustConfig(r.Context(), o.homeVerified)
		ended := make(chan error, 2)
		go func() { ended <- forward(down, up, func(data []byte) { o.request(data); trust.request(data) }) }()
		go func() { ended <- forward(up, down, o.response, trust.response) }()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		runFrontendLifetime(ctx, tick.C, down, up, ended, binding)
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
		launch.publish(0)
		fmt.Fprintln(os.Stderr, "gtmux:", err)
		return 1
	}
	launch.publish(cmd.Process.Pid)
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

func forward(from, to *websocket.Conn, observe func([]byte), transforms ...func([]byte) []byte) error {
	for {
		kind, body, err := from.ReadMessage()
		if err != nil {
			return err
		}
		if kind == websocket.TextMessage {
			for _, transform := range transforms {
				body = transform(body)
			}
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

// Use the protected short-socket layout without replacing the real daemon.
// An explicit non-default Unix endpoint is still Remote in Codex; worktree
// trust compatibility is handled separately after verified same-home initialize.
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
