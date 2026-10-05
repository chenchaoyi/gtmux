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

// GuestAttachRefused is the refusal a share link gets from /api/attach; see handleAttach.
const GuestAttachRefused = "forbidden: a share link cannot open a terminal: it would show the whole tmux session, not only the shared panes. Open the link in a browser instead."

// attachRecheckInterval is how often an open terminal re-checks the caller's token, so a
// revoked caller's session ends within it rather than whenever the caller detaches.
var attachRecheckInterval = 2 * time.Second // a test shortens it

// attachUpgrader upgrades /api/attach to a WebSocket. The bearer token (checked by
// auth() before we get here) is the security boundary, so Origin is not gated.
var attachUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// handleAttach bridges a tmux pane's PTY to a WebSocket (the `gtmux attach` client), for
// the owner and paired devices only. The pane→tmux-client command is injected
// (AttachCommand) so this stays decoupled from tmux.
func (s *Server) handleAttach(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	// A share link cannot open a terminal. The bridge runs a tmux CLIENT attached to the
	// pane's session, and a link grants panes, not sessions. Reproduced on an isolated
	// serve (2026-10-06): a link granted one pane received its window's other, unshared
	// pane on the first frame; and a link that may type pressed the tmux prefix, opened
	// the command prompt and switched the client to another session, whose output then
	// streamed. The old gate checked the requested pane and dropped a view-only guest's
	// input, which bounded neither. Until the bridge can carry one pane alone, a guest has
	// the browser and phone views, which are scoped per pane.
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody(GuestAttachRefused))
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
	end := func() {
		endOnce.Do(func() {
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
		"via", via(r))
	defer func() {
		lg.Info("attach.closed", "a remote terminal session ended", "pane", id,
			"actor", actorOf(r.Context()), "seconds", int(time.Since(opened).Seconds()))
	}()

	done := make(chan struct{})
	// gorilla/websocket forbids concurrent writes, and the cursor sampler below writes
	// alongside the output pump. Output takes the lock normally; the sampler only
	// TryLocks, so a cursor frame can NEVER delay or block the PTY stream.
	var wmu sync.Mutex

	// PTY → WS: read raw pane bytes and send OUTPUT frames. WriteMessage is
	// synchronous, so a slow client backpressures this read (TCP → pty → tmux),
	// bounding memory without an explicit queue. Ends when the pty closes (detach/exit).
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
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
					wmu.Lock()
					_ = conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, []byte("\r\n[gtmux] access revoked\r\n")))
					wmu.Unlock()
					end()
					return
				}
			}
		}()
	}

	// WS → PTY: input + resize. Runs in its own goroutine so input (e.g. Ctrl-C) is
	// never blocked behind output backpressure.
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
				_, _ = ptmx.Write(payload)
			case connect.OpResize:
				if cols, rows, ok := connect.DecodeResize(payload); ok {
					_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
				}
				// PAUSE/RESUME: natural backpressure (synchronous WriteMessage) already
				// bounds memory for a raw-terminal client, so the MVP treats them as
				// no-ops; a future async client can drive explicit flow control here.
			}
		}
	}()

	<-done
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
