// Package server exposes gtmux's read-only agent radar over HTTP for the remote
// mobile app. It is pure transport: every tmux/agent capability is injected via
// Deps, so this package never imports internal/app (no import cycle) and stays
// unit-testable with fakes.
//
// Routes:
//
//	GET  /api/health        — liveness/reachability probe (unauthenticated)
//	GET  /api/agents        — the `gtmux agents --json` array (byte-identical)
//	GET  /api/pane?id=%N    — a pane's current screen text (read-only)
//	POST /api/focus?id=%N   — select that pane locally (no input injection)
//	POST /api/send          — type text / a control key into a pane (WRITE)
//
// Security model: every /api/* route (except health) requires a Bearer token,
// compared in constant time. /api/send WRITES to a terminal (tmux send-keys), so
// the token now also gates terminal input — a leaked token allows running
// commands on the Mac. Keep the token secret and the tunnel deliberate.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/terminal"
)

// Deps are the read-only capabilities the HTTP layer needs. The caller
// (internal/app) supplies them from gtmux's existing internals, so this package
// stays decoupled from app and easy to test with fakes.
type Deps struct {
	// AgentsJSON returns the marshaled agents array — byte-identical to
	// `gtmux agents --json` — so app, menu-bar app, and mobile app share one
	// contract. It should return an empty JSON array (not an error) when no
	// tmux server is running.
	AgentsJSON func() ([]byte, error)

	// PaneText returns a pane's current screen text (read-only capture-pane).
	// ok is false when the pane no longer exists.
	PaneText func(id string) (text string, ok bool)

	// IsWaiting reports whether the pane is blocked on the user per the HOOK marker
	// (never inferred from screen text). handleOptions consults it so the approval
	// card's 1/2/3 choices are only ever parsed for a genuinely-waiting pane — a
	// numbered list in ordinary output must never surface as an approval menu.
	// Optional: nil → no gate (the parse runs unconditionally, legacy behavior).
	IsWaiting func(id string) bool

	// PaneCursor returns the pane's text cursor: column x (0-based) and Up = rows
	// above the last captured line, plus whether it's visible. Optional: nil → the
	// pane response omits the cursor (the renderer then shows none / its own).
	PaneCursor func(id string) (x, up int, visible, ok bool)

	// Focus selects a pane locally — the "back at your desk, you're already on
	// it" action. It injects no input. err is non-nil if the pane is gone.
	Focus func(id string) error

	// Send types into a pane (WRITE). Exactly one of text/key is used: a non-empty
	// key sends that NAMED key (validated against an allowlist by the impl); else
	// text is typed literally, plus Enter when enter is true. err if the pane is
	// gone or the key isn't allowed. Optional: nil → POST /api/send returns 503.
	Send func(id, text, key string, enter bool) error

	// Upload saves an uploaded file on the Mac and returns its local path (so the
	// phone can hand a photo/file to an agent by path). Optional: nil → POST
	// /api/upload returns 503.
	Upload func(name string, data []byte) (path string, err error)

	// Icon returns a PNG of the named agent's identity icon (from the user's
	// installed app, like the menu-bar app — nothing bundled), or nil. Optional.
	Icon func(agent string) []byte

	// Diff returns a unified `git diff` (working tree vs HEAD, plus untracked) of
	// the pane's current working directory — "what did the agent change". Empty
	// string when the cwd isn't a git repo. Optional: nil → GET /api/diff is 503.
	Diff func(id string) (diff string, err error)

	// Theme returns the active host terminal's resolved appearance (colors + font)
	// so the pane mirror can match the user's real terminal. Optional: nil → GET
	// /api/theme is 503.
	Theme func() terminal.Theme

	// Transcript returns the marshaled chat-history turns for a pane (the agent's
	// conversation parsed into prompt → collapsed steps → final response), as a
	// JSON array. Empty array (not error) when the pane has no resumable session
	// or the agent's log isn't found. Optional: nil → GET /api/transcript is 503.
	Transcript func(id string) ([]byte, error)

	// AgentStatuses returns a lean snapshot of current agents for the SSE loop
	// to diff (status transitions → `alert` events + push). Optional: if nil,
	// GET /api/events still serves heartbeats but emits no agents/alert events.
	AgentStatuses func() []AgentStatus

	// OnAlert, if set, is called for every waiting/done transition the events
	// loop detects — the hook a push manager uses to forward alerts to the relay
	// without re-deriving them. Optional. Ignored when Push is set (the manager's
	// OnAlert is used instead).
	OnAlert func(Alert)

	// Push, if set, enables POST /api/push/register and receives every alert for
	// forwarding to the relay. Optional: when nil, push registration returns 503.
	Push *PushManager

	// OnClients, if set, is called with the live roster of connected SSE clients
	// (each phone/browser actively viewing) whenever it changes, plus a heartbeat on
	// each tick while non-empty — so the menu-bar app can show WHO is connected and
	// detect a dead serve via staleness. Optional.
	OnClients func([]ClientInfo)

	// Enroll, if set, lets a phone pair via a short-lived code (POST /api/enroll)
	// for its own per-device token, which auth then accepts alongside the master
	// token. Optional: when nil, /api/enroll is 503 and only the master token works.
	Enroll *EnrollManager
}

// Config configures the listener and auth token.
type Config struct {
	Addr  string // host:port to bind, e.g. "0.0.0.0:8765"
	Token string // required Bearer token for every /api/* route (except health)
}

// Server wraps Config + Deps into an http.Handler and a listener.
type Server struct {
	cfg  Config
	deps Deps
	hub  *hub
}

// New returns a Server. cfg.Token must be non-empty (callers generate one).
func New(cfg Config, deps Deps) *Server {
	onAlert := deps.OnAlert
	if deps.Push != nil { // the push manager forwards alerts to the relay
		onAlert = deps.Push.OnAlert
	}
	s := &Server{
		cfg:  cfg,
		deps: deps,
		hub:  newHub(deps.AgentStatuses, eventsInterval, onAlert),
	}
	if deps.Push != nil { // on every tally change: Live Activity update + silent badge sync
		s.hub.onTally = deps.Push.OnTally
	}
	s.hub.onClients = deps.OnClients // remote-viewer indicator (count of live SSE clients)
	return s
}

// MintEnroll returns a fresh short-lived single-use pairing code (for a browser
// pairing link), or "" if enrollment isn't configured.
func (s *Server) MintEnroll() string {
	if s.deps.Enroll == nil {
		return ""
	}
	return s.deps.Enroll.Mint()
}

// Handler builds the routed, token-guarded http.Handler (exposed for tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth) // unauthenticated probe
	mux.HandleFunc("/api/enroll", s.handleEnroll) // unauthenticated: the short-lived code IS the credential
	mux.Handle("/api/enroll/mint", s.auth(http.HandlerFunc(s.handleEnrollMint)))
	mux.Handle("/api/devices", s.auth(http.HandlerFunc(s.handleDevices)))
	mux.Handle("/api/devices/revoke", s.auth(http.HandlerFunc(s.handleRevoke)))
	mux.Handle("/api/agents", s.auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("/api/pane", s.auth(http.HandlerFunc(s.handlePane)))
	mux.Handle("/api/options", s.auth(http.HandlerFunc(s.handleOptions)))
	mux.Handle("/api/focus", s.auth(http.HandlerFunc(s.handleFocus)))
	mux.Handle("/api/send", s.auth(http.HandlerFunc(s.handleSend)))
	mux.Handle("/api/upload", s.auth(http.HandlerFunc(s.handleUpload)))
	mux.Handle("/api/icon", s.auth(http.HandlerFunc(s.handleIcon)))
	mux.Handle("/api/diff", s.auth(http.HandlerFunc(s.handleDiff)))
	mux.Handle("/api/transcript", s.auth(http.HandlerFunc(s.handleTranscript)))
	mux.Handle("/api/theme", s.auth(http.HandlerFunc(s.handleTheme)))
	mux.Handle("/api/events", s.auth(http.HandlerFunc(s.handleEvents)))
	mux.Handle("/api/push/register", s.auth(http.HandlerFunc(s.handleRegister)))
	mux.Handle("/api/push/activity", s.auth(http.HandlerFunc(s.handleActivityRegister)))
	mux.Handle("/api/push/test", s.auth(http.HandlerFunc(s.handleTest)))
	// Browser-mirror web UI (view-only, unauthenticated static page). Registered
	// at "/" so the specific /api/* patterns take precedence; this only ever serves
	// non-API paths (index.html, app.js, style.css, vendor/*).
	mux.Handle("/", webHandler())
	return mux
}

// ListenAndServe starts the SSE diff loop, then binds cfg.Addr and serves until
// error. The loop runs for the lifetime of the process.
func (s *Server) ListenAndServe() error {
	go s.hub.run(context.Background())
	return http.ListenAndServe(s.cfg.Addr, s.Handler())
}

// auth wraps next with constant-time Bearer-token verification.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		tok := ""
		if strings.HasPrefix(h, prefix) {
			tok = strings.TrimPrefix(h, prefix)
		}
		master := subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.Token)) == 1
		if !master && !(s.deps.Enroll != nil && s.deps.Enroll.ValidToken(tok)) {
			writeJSON(w, http.StatusUnauthorized, errBody("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "gtmux"})
}

func (s *Server) handleAgents(w http.ResponseWriter, _ *http.Request) {
	b, err := s.deps.AgentsJSON()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("agents error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// paneResponse is the JSON shape of GET /api/pane.
type paneResponse struct {
	ID     string      `json:"id"`
	Text   string      `json:"text"`
	Cursor *paneCursor `json:"cursor,omitempty"`
}

// paneCursor is the pane's text cursor, positioned RELATIVE TO THE BOTTOM so it
// survives the phone's terminal having a different row count than the Mac pane:
// Up = rows up from the last captured line, X = column (0-based), Visible = the
// pane's cursor flag (false in alt-screen TUIs that hide it).
type paneCursor struct {
	X       int  `json:"x"`
	Up      int  `json:"up"`
	Visible bool `json:"visible"`
}

func (s *Server) handlePane(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	text, ok := s.deps.PaneText(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("pane not found"))
		return
	}
	resp := paneResponse{ID: id, Text: text}
	if s.deps.PaneCursor != nil {
		if x, up, vis, ok := s.deps.PaneCursor(id); ok {
			resp.Cursor = &paneCursor{X: x, Up: up, Visible: vis}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOptions returns a waiting pane's interactive 1/2/3 choice block as JSON
// ({"options":[{n,label}…]}) using the SAME parser as the menu-bar / `gtmux
// options` (HANDOFF: one shared parser). The mobile approval card renders these.
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	// Options belong to a HOOK-confirmed waiting pane only — never derive an approval
	// menu from arbitrary screen text (a "1. … 2. …" list in an agent's own message
	// would otherwise read as one). Not waiting → empty, without even parsing.
	if s.deps.IsWaiting != nil && !s.deps.IsWaiting(id) {
		writeJSON(w, http.StatusOK, map[string]any{"options": []prompt.Option{}})
		return
	}
	text, ok := s.deps.PaneText(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("pane not found"))
		return
	}
	opts := prompt.ParseOptions(text)
	if opts == nil {
		opts = []prompt.Option{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": opts})
}

func (s *Server) handleFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	if err := s.deps.Focus(id); err != nil {
		writeJSON(w, http.StatusNotFound, errBody("focus failed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// sendRequest is the JSON body of POST /api/send. Provide `key` for a named
// control key (Enter, C-c, …) OR `text` for literal input (+ `enter`).
type sendRequest struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Key   string `json:"key"`
	Enter bool   `json:"enter"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Send == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("input not available"))
		return
	}
	var req sendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("bad json"))
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	if req.Key == "" && req.Text == "" && !req.Enter {
		writeJSON(w, http.StatusBadRequest, errBody("nothing to send"))
		return
	}
	if err := s.deps.Send(req.ID, req.Text, req.Key, req.Enter); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("send failed: "+err.Error()))
		return
	}
	// Return the freshly-redrawn screen WITH the send so the client renders the echo
	// in a SINGLE round-trip instead of a separate /api/pane fetch — the big latency
	// win over a remote tunnel (two RTTs → one). Settle briefly first so the agent's
	// TUI has a frame to redraw the typed input into the captured screen.
	resp := sendResponse{Status: "ok"}
	if s.deps.PaneText != nil {
		if sendSettle > 0 {
			time.Sleep(sendSettle)
		}
		if text, ok := s.deps.PaneText(req.ID); ok {
			resp.Text = text
			if s.deps.PaneCursor != nil {
				if x, up, vis, ok := s.deps.PaneCursor(req.ID); ok {
					resp.Cursor = &paneCursor{X: x, Up: up, Visible: vis}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// sendSettle is how long handleSend waits after send-keys before snapshotting the
// pane, giving the agent's TUI a frame to redraw the echoed input into the returned
// screen. A var so tests can zero it.
var sendSettle = 90 * time.Millisecond

// sendResponse is POST /api/send's reply: the post-send pane snapshot (text +
// cursor) so the client shows the echo in one round-trip. `status` stays for
// back-compat with older clients that only checked it; `text`/`cursor` are omitted
// when the pane couldn't be read (the client then falls back to its poll).
type sendResponse struct {
	Status string      `json:"status"`
	Text   string      `json:"text,omitempty"`
	Cursor *paneCursor `json:"cursor,omitempty"`
}

// handleUpload accepts a multipart "file" and saves it on the Mac (≤ 30 MB),
// returning {"path": "..."} so the client can reference it to an agent.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Upload == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("upload not available"))
		return
	}
	const maxBytes = 30 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("bad upload (too large?)"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("missing file"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("read failed"))
		return
	}
	path, err := s.deps.Upload(header.Filename, data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("save failed: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

// handleTheme serves the active host terminal's resolved appearance (GET
// /api/theme) so the pane mirror matches the user's real terminal. Read per
// request, so editing the terminal's config is reflected on the next fetch.
func (s *Server) handleTheme(w http.ResponseWriter, r *http.Request) {
	if s.deps.Theme == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("theme not available"))
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Theme())
}

// handleIcon serves a PNG of an agent's identity icon (GET /api/icon?agent=NAME).
func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	if s.deps.Icon == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("icons not available"))
		return
	}
	png := s.deps.Icon(r.URL.Query().Get("agent"))
	if png == nil {
		writeJSON(w, http.StatusNotFound, errBody("no icon"))
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(png)
}

// diffResponse is the JSON shape of GET /api/diff.
type diffResponse struct {
	ID   string `json:"id"`
	Diff string `json:"diff"` // unified diff text; "" when the cwd isn't a git repo
}

// handleDiff serves a unified `git diff` of the pane's cwd (GET /api/diff?id=%N) —
// "what did the agent change". Read-only.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if s.deps.Diff == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("diff not available"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	diff, err := s.deps.Diff(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errBody("diff failed: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, diffResponse{ID: id, Diff: diff})
}

// handleTranscript serves the pane's parsed chat history (GET /api/transcript?id=%N)
// as a JSON array of turns. Read-only.
func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	if s.deps.Transcript == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("transcript not available"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("missing id"))
		return
	}
	b, err := s.deps.Transcript(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errBody("transcript failed: "+err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// handleTest sends a test push to every registered device (POST /api/push/test),
// so the settings screen can verify notifications end-to-end.
func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if s.deps.Push == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("push not configured"))
		return
	}
	n := s.deps.Push.Test()
	writeJSON(w, http.StatusOK, map[string]int{"sent": n})
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

// writeJSON writes v as JSON with the given status. Best-effort: a write error
// after the header is sent can't be reported to the client.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
