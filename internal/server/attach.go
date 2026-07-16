package server

import (
	"net/http"
	"os"
	"os/exec"

	"github.com/chenchaoyi/gtmux/internal/connect"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

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
		(hasDev && s.deps.Share != nil && s.deps.Share.InputEnabled() && dev.MayInput(id))

	conn, err := attachUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error
	}
	defer conn.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = attachEnv(resolveTerm(r.URL.Query().Get("term")))
	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, []byte("\r\n[gtmux] attach failed: "+err.Error()+"\r\n")))
		return
	}
	// Killing the tmux client detaches (the session lives on); close the pty so both
	// bridge goroutines unwind.
	defer func() { _ = ptmx.Close() }()

	done := make(chan struct{})

	// PTY → WS: read raw pane bytes and send OUTPUT frames. WriteMessage is
	// synchronous, so a slow client backpressures this read (TCP → pty → tmux),
	// bounding memory without an explicit queue. Ends when the pty closes (detach/exit).
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, connect.Encode(connect.OpOutput, buf[:n])); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WS → PTY: input + resize (dropped for a read-only pane). Runs in its own
	// goroutine so input (e.g. Ctrl-C) is never blocked behind output backpressure.
	go func() {
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				_ = ptmx.Close() // unblock the reader
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
