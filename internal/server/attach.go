package server

import (
	"crypto/subtle"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/chenchaoyi/gtmux/internal/connect"
	"github.com/chenchaoyi/gtmux/internal/diag"
)

// attachCursorInterval paces the OpCursor sampler — fast enough to reconcile a
// prediction quickly, slow enough that a `display-message` per tick is negligible.
const attachCursorInterval = 120 * time.Millisecond

// attachRecheckInterval is how often an open terminal re-checks the caller's token, so a
// revoked caller's session ends within it rather than whenever the caller detaches.
var attachRecheckInterval = 2 * time.Second // a test shortens it

// attachNoticeWait bounds the "access revoked" notice: how long it may wait for the
// output pump's write lock, and then how long its own write may take. A test shortens it.
var attachNoticeWait = 500 * time.Millisecond

// attachUpgrader upgrades /api/attach to a WebSocket. The bearer token (checked by
// auth() before we get here) is the security boundary, so Origin is not gated.
var attachUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// handleAttach bridges a tmux pane's PTY to a WebSocket (the `gtmux attach` client).
// Scope is enforced HERE, before any PTY is spawned: an owner may attach any pane; a
// guest may attach ONLY a view-allowed pane, and INPUT/RESIZE frames are dropped for a
// pane it may not type into (a view-only pane is read-only). The pane→tmux-client
// command is injected (AttachCommand) so this stays decoupled from tmux.
func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	scope := callerScope(r.Context())
	// A guest may attach ONLY a pane ITS OWN link may VIEW (pair-share-model);
	// refuse the upgrade otherwise (no PTY is ever spawned outside the link's scope).
	dev, hasDev := callerDevice(r.Context())
	if scope == scopeGuest && (!hasDev || !dev.MayView(id)) {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: pane not shared"))
		return
	}
	// Pane grants are only meaningful within the tmux server they were made against —
	// after a restart the ids are reassigned, so "%17" may now be an unrelated pane.
	// Fail CLOSED until the owner re-grants (share-grant-epoch).
	if scope == scopeGuest && s.deps.Share != nil && s.deps.Share.GrantsStale() {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: share is stale (tmux restarted) — the owner must re-grant"))
		return
	}
	if s.deps.AttachCommand == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("attach not configured"))
		return
	}
	argv, ok := s.deps.AttachCommand(id)
	if !ok || len(argv) == 0 {
		writeJSON(w, http.StatusNotFound, errBody("pane not found"))
		return
	}
	// canType: an owner types anywhere; a guest only with host consent AND the pane
	// on ITS OWN input allowlist (a view-only pane is read-only — write frames drop).
	canType := scope != scopeGuest ||
		(hasDev && s.deps.Share != nil && s.deps.Share.InputEnabled() &&
			!s.deps.Share.GrantsStale() && dev.MayInput(id))

	conn, err := attachUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error
	}
	defer conn.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = attachEnv(resolveTerm(r.URL.Query().Get("term")))
	ptmx, err := pty.Start(cmd)
	if err != nil {
		lg.Act("act.attach", actorOf(r.Context()), id, diag.Failed, "a remote terminal session could not start", "error", err)
		_ = conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, []byte("\r\n[gtmux] attach failed: "+err.Error()+"\r\n")))
		return
	}
	// end stops the session from whichever side gives out first: the WebSocket, the tmux
	// client, its PTY. Closing the PTY alone did not: on macOS a read blocked on a quiet
	// terminal is not woken by it, so a revoked session with nothing to print stayed open
	// with its notice sent (%12's re-verification, 2026-10-06), and a client that dropped
	// off a quiet pane left its handler and tmux client behind. Ending the tmux client
	// closes the terminal's other side, and the read returns. Killing it only detaches:
	// the session lives on.
	var endOnce sync.Once
	stop := make(chan struct{}) // closed by end: releases a pump waiting on a PAUSE
	end := func() {
		endOnce.Do(func() {
			close(stop)
			_ = conn.Close()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = ptmx.Close()
		})
	}
	defer func() {
		end()
		_ = cmd.Wait() // reap the tmux client
	}()
	// Who opened a terminal on this Mac, into which pane, and for how long.
	opened := time.Now()
	lg.Act("act.attach", actorOf(r.Context()), id, diag.OK, "a remote terminal session opened",
		"can_type", canType, "via", via(r))
	defer func() {
		lg.Info("attach.closed", "a remote terminal session ended", "pane", id,
			"actor", actorOf(r.Context()), "seconds", int(time.Since(opened).Seconds()))
	}()

	done := make(chan struct{})
	// gorilla/websocket forbids concurrent writes, and the cursor sampler below writes
	// alongside the output pump. Output takes the lock normally; the sampler only
	// TryLocks, so a cursor frame can NEVER delay or block the PTY stream.
	var wmu sync.Mutex

	// flow is the client's PAUSE/RESUME, per connection (remote-terminal-client: the
	// bridge honors them).
	flow := newFlowGate()

	// PTY → WS: read raw pane bytes and send OUTPUT frames. WriteMessage is
	// synchronous, so a slow client backpressures this read (TCP → pty → tmux),
	// bounding memory without an explicit queue. Ends when the pty closes (detach/exit).
	// A PAUSE is honored at both ends of a read: no read starts while paused, and bytes
	// a read already returned are held (at most one buffer) until RESUME rather than
	// sent. A frame already being written when PAUSE arrives completes. The wait never
	// holds wmu, and end() releases it.
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			if !flow.wait(stop) {
				return
			}
			n, err := ptmx.Read(buf)
			if n > 0 {
				if !flow.wait(stop) {
					return
				}
				wmu.Lock()
				werr := conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, buf[:n]))
				wmu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// Cursor sampler → WS (attach-predictive-echo): stream the pane's tmux cursor +
	// alt-screen flag so the client has the AUTHORITATIVE cursor without emulating a
	// terminal. Coalesced (sent only when it changed) and TryLock'd (skipped whenever
	// the output pump is mid-write) — a dropped sample is harmless, the next tick
	// carries it. Absent the dep, no frames are sent and clients never predict.
	if s.deps.AttachCursor != nil {
		go func() {
			t := time.NewTicker(attachCursorInterval)
			defer t.Stop()
			var last connect.Cursor
			var have bool
			for {
				select {
				case <-done:
					return
				case <-t.C:
					x, y, alt, ok := s.deps.AttachCursor(id)
					if !ok {
						continue
					}
					c := connect.Cursor{X: x, Y: y, Alt: alt}
					if have && c == last {
						continue // unchanged → don't spend a frame
					}
					if flow.paused() {
						continue // the client asked for nothing more until RESUME
					}
					if !wmu.TryLock() {
						continue // output is writing; never queue behind it
					}
					werr := conn.WriteMessage(websocket.BinaryMessage, connect.EncodeCursor(x, y, alt))
					wmu.Unlock()
					if werr != nil {
						return
					}
					last, have = c, true
				}
			}
		}()
	}

	// Revoking a device or a share link ends its open terminal too. auth() checks the
	// token once, before the upgrade: measured on an isolated serve (2026-10-06), a
	// revoked link's and a revoked device's sessions both kept streaming the pane, while
	// any new request with the same token was already refused. The serve's own token
	// does not change while it runs, so only enrolled tokens are re-checked.
	if tok := bearerToken(r); tok != "" && s.deps.Enroll != nil &&
		subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.Token)) != 1 {
		go func() {
			t := time.NewTicker(attachRecheckInterval)
			defer t.Stop()
			for {
				select {
				case <-done:
					return
				case <-t.C:
					if _, ok := s.deps.Enroll.TokenScope(tok); ok {
						continue
					}
					lg.Act("act.attach", actorOf(r.Context()), id, diag.OK, "a remote terminal session was closed: its access was revoked")
					// The notice is best effort and bounded. The output pump holds wmu
					// for a whole write, and a client that stops reading keeps that
					// write open (TCP backpressure), so waiting for the lock kept a
					// revoked session open for as long as its client stalled (%12's
					// reproduction, 2026-10-06). The session ends whether or not the
					// notice was written: closing the connection is what releases a
					// stuck write.
					if tryLockFor(&wmu, attachNoticeWait) {
						_ = conn.SetWriteDeadline(time.Now().Add(attachNoticeWait))
						_ = conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, []byte("\r\n[gtmux] access revoked\r\n")))
						wmu.Unlock()
					}
					end()
					return
				}
			}
		}()
	}

	// WS → PTY: input + resize (dropped for a read-only pane). Runs in its own
	// goroutine so input (e.g. Ctrl-C) is never blocked behind output backpressure.
	go func() {
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				end()
				return
			}
			if mt != websocket.BinaryMessage {
				continue
			}
			op, payload, ok := connect.Decode(data)
			if !ok {
				continue
			}
			switch op {
			case connect.OpInput:
				if canType {
					_, _ = ptmx.Write(payload)
				}
			case connect.OpResize:
				if canType {
					if cols, rows, ok := connect.DecodeResize(payload); ok {
						_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
					}
				}
			case connect.OpPause:
				// Flow control, not input: any caller may pause its own stream.
				flow.pause()
			case connect.OpResume:
				flow.resume()
			}
		}
	}()

	<-done
}

// flowGate is one attach connection's PAUSE/RESUME state. Repeated frames are
// idempotent, and a RESUME without a PAUSE does nothing.
type flowGate struct {
	mu   sync.Mutex
	open chan struct{} // closed while the stream may flow; replaced by a PAUSE
}

func newFlowGate() *flowGate {
	g := &flowGate{open: make(chan struct{})}
	close(g.open)
	return g
}

func (g *flowGate) pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.open:
		g.open = make(chan struct{})
	default: // already paused
	}
}

func (g *flowGate) resume() {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.open: // already flowing
	default:
		close(g.open)
	}
}

func (g *flowGate) paused() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.open:
		return false
	default:
		return true
	}
}

// wait returns true once the stream may flow, or false when stop closes first.
func (g *flowGate) wait(stop <-chan struct{}) bool {
	g.mu.Lock()
	open := g.open
	g.mu.Unlock()
	select {
	case <-open:
		return true
	case <-stop:
		return false
	}
}

// tryLockFor takes mu if it comes free within d, and reports whether it did.
func tryLockFor(mu *sync.Mutex, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for !mu.TryLock() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// resolveTerm picks the TERM for the tmux client spawned in the PTY. Prefer the
// CLIENT's own $TERM (sent by `gtmux attach`) so the user's real terminal is honored
// (truecolor, Ghostty/kitty features, …) — but ONLY when the remote has terminfo for
// it, else tmux dies with "terminal does not support clear". Fall back to a
// widely-supported terminfo otherwise. The serve's launchd env has no TERM at all.
func resolveTerm(clientTerm string) string {
	clientTerm = sanitizeTerm(clientTerm)
	if clientTerm != "" && termExists(clientTerm) {
		return clientTerm
	}
	return "xterm-256color"
}

// termExists reports whether the remote has a terminfo entry for term (via infocmp).
func termExists(term string) bool {
	return exec.Command("infocmp", term).Run() == nil
}

// sanitizeTerm keeps only terminfo-name-safe characters — a client-supplied TERM
// becomes an env var of a spawned process, so never let arbitrary bytes through.
func sanitizeTerm(s string) string {
	if s == "" || len(s) > 64 {
		return ""
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '.' || r == '+' || r == '_') {
			return ""
		}
	}
	return s
}

// attachEnv is the environment for the tmux client spawned in the PTY: the resolved
// TERM, plus a UTF-8 locale. The serve's launchd env has NO locale, so without this
// (and `-u` on the tmux command) CJK / wide chars render as placeholder dashes instead
// of the actual glyphs. Matches the internal/tmux UTF-8 fix used for the radar.
func attachEnv(term string) []string {
	return append(os.Environ(), "TERM="+term, "LANG=en_US.UTF-8", "LC_CTYPE=en_US.UTF-8")
}
