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
//
// Caller SCOPE (web-shared-input): auth() resolves each token to a scope — master,
// device (a paired phone), or guest (a share link) — and carries it in the request
// context. master/device have full input; a GUEST's /api/send is gated by the share
// policy (host consent + a per-pane allowlist, POST /api/share/config, master-only),
// so a shared link can type only into the panes the host chose. GET /api/share returns
// the caller's own input capability so a UI mirrors (never widens) the server gate.
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

	// AttachCursor returns the pane's cursor CELL (x,y) plus whether the pane is on
	// the ALTERNATE screen — the attach bridge streams this as OpCursor frames so the
	// attach client gets the authoritative cursor without emulating a terminal
	// (attach-predictive-echo). `alt` is the precise "full-screen TUI" signal that a
	// mere cursor-visible flag can't give (vim shows a cursor). Optional: nil → the
	// bridge sends no cursor frames and clients simply never predict.
	AttachCursor func(id string) (x, y int, alt, ok bool)

	// Focus selects a pane locally — the "back at your desk, you're already on
	// it" action. It injects no input. err is non-nil if the pane is gone.
	Focus func(id string) error

	// AttachCommand returns the argv of a tmux CLIENT to spawn in a server-side PTY
	// for `GET /api/attach` (e.g. `tmux attach-session -t <session-of-pane>`), so the
	// handler can bridge that PTY to the WebSocket. ok=false when the pane is gone.
	// Optional: nil → /api/attach returns 503. Injected so the server stays decoupled
	// from tmux (the pane→session resolution lives in the wiring).
	AttachCommand func(paneID string) (argv []string, ok bool)

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

	// HQBoard returns the supervisor's situation board — the synthesis it maintains by
	// hand so its picture of the fleet survives a context reset — plus when it was last
	// written (unix secs). ok=false means no board exists, which is an ordinary state
	// (a supervisor that has never written one), NOT an error. Optional: nil → GET
	// /api/hq/board reports no board rather than 503, since "no board" is the same
	// answer either way and a client shouldn't need to tell the two apart.
	HQBoard func() (text string, modUnix int64, ok bool)

	// HQEvents returns the marshaled event ledger at a severity FLOOR ("" = all),
	// newest first, capped at limit records — the fleet's recent history, which the
	// radar's present-instant view cannot show. Optional: nil → GET /api/hq/events
	// serves an empty array (a client renders "no activity", not an error).
	HQEvents func(severity string, limit int) ([]byte, error)

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
	// Share, if set, holds the shared-input policy (consent + per-pane allowlist) that
	// gates a GUEST token's /api/send. Optional: nil → guests can never type.
	Share *ShareManager

	// DigestJSON returns the marshaled agent-digest array — byte-identical to
	// `gtmux digest --json` (the supervisor's fleet view: goal/last/ask per row).
	// Additive to AgentsJSON. Optional: nil → GET /api/digest is 503.
	DigestJSON func() ([]byte, error)

	// UsageJSON returns the marshaled usage report — byte-identical to
	// `gtmux usage --json` (usage-watch). Optional: nil → GET /api/usage is 503.
	UsageJSON func() ([]byte, error)

	// OnSlowTick, if set, is called on a slow cadence (~20s) from the hub's single
	// goroutine — the SINGLE-WRITER place to sample resources + evaluate limits and
	// emit resource·warn / limits·warn nudges without the read-check-write race a
	// getter-invoked-by-many-callers has. Optional.
	OnSlowTick func()

	// OnFastTick, if set, is called on a fast cadence (~3s) from the same single
	// goroutine. It exists for work that must feel immediate and costs ~nothing when
	// there is none: the HQ nudge drain (a knock queued behind a half-typed draft
	// must land in seconds, and the slow tick's cadence is paced by df/ps sampling
	// that has nothing to do with it). Keep whatever runs here cheaply gated.
	// Optional.
	OnFastTick func()
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
	s.hub.onClients = deps.OnClients   // remote-viewer indicator (count of live SSE clients)
	s.hub.onSlowTick = deps.OnSlowTick // single-writer resource/limits evaluator + nudge
	s.hub.onFastTick = deps.OnFastTick // single-writer HQ nudge drain (must feel immediate)
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
	mux.Handle("/api/share", s.auth(http.HandlerFunc(s.handleShare)))              // any: the caller's capability
	mux.Handle("/api/share/config", s.auth(http.HandlerFunc(s.handleShareConfig))) // master: consent + allowlist
	mux.Handle("/api/share/new", s.auth(http.HandlerFunc(s.handleShareNew)))       // master: mint a guest link
	mux.Handle("/api/share/set", s.auth(http.HandlerFunc(s.handleShareSet)))       // full: edit ONE link's scope
	mux.Handle("/api/share/link", s.auth(http.HandlerFunc(s.handleShareLink)))     // full: re-copy a link's URL
	mux.Handle("/api/agents", s.auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("/api/digest", s.auth(http.HandlerFunc(s.handleDigest)))
	mux.Handle("/api/usage", s.auth(http.HandlerFunc(s.handleUsage)))
	mux.Handle("/api/hq/board", s.auth(http.HandlerFunc(s.handleHQBoard)))   // owner: the supervisor's situation board
	mux.Handle("/api/hq/events", s.auth(http.HandlerFunc(s.handleHQEvents))) // owner: severity-floored event ledger
	mux.Handle("/api/pane", s.auth(http.HandlerFunc(s.handlePane)))
	mux.Handle("/api/attach", s.auth(http.HandlerFunc(s.handleAttach))) // WS: raw PTY attach, scope-gated
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
	mux.Handle("/api/push/unregister", s.auth(http.HandlerFunc(s.handleUnregister)))
	mux.Handle("/api/push/activity", s.auth(http.HandlerFunc(s.handleActivityRegister)))
	mux.Handle("/api/push/test", s.auth(http.HandlerFunc(s.handleTest)))
	mux.Handle("/api/push/tokens", s.auth(http.HandlerFunc(s.handleTokens)))
	mux.Handle("/api/push/forget", s.auth(http.HandlerFunc(s.handleForget)))
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
// Caller scopes carried in the request context by auth(): the host's own master
// token, a paired device (the owner's phone — full input), or a guest share link
// (input-restricted by the share gate).
const (
	scopeMaster = "master"
	scopeDevice = "device"
	scopeGuest  = "guest"
)

type ctxKey int

const (
	scopeCtxKey  ctxKey = 0
	deviceCtxKey ctxKey = 1
)

// callerScope returns the authenticated caller's scope ("" if unset).
func callerScope(ctx context.Context) string {
	s, _ := ctx.Value(scopeCtxKey).(string)
	return s
}

// callerDevice returns the authenticated GUEST caller's roster entry — the link
// whose per-link scope gates its reads/inputs (pair-share-model). ok=false for
// master/device callers (unrestricted) and when unset.
func callerDevice(ctx context.Context) (EnrolledDevice, bool) {
	d, ok := ctx.Value(deviceCtxKey).(EnrolledDevice)
	return d, ok
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		tok := ""
		if strings.HasPrefix(h, prefix) {
			tok = strings.TrimPrefix(h, prefix)
		}
		scope := scopeMaster
		ctx := r.Context()
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.Token)) != 1 {
			sc, ok := "", false
			if s.deps.Enroll != nil {
				sc, ok = s.deps.Enroll.TokenScope(tok)
			}
			if !ok {
				writeJSON(w, http.StatusUnauthorized, errBody("unauthorized"))
				return
			}
			scope = sc // scopeDevice | scopeGuest
			// A guest's request carries ITS link (per-link scope) so every gate
			// downstream checks the caller's own allowlists, not a global set.
			if scope == scopeGuest {
				if d, ok := s.deps.Enroll.DeviceByToken(tok); ok {
					ctx = context.WithValue(ctx, deviceCtxKey, d)
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, scopeCtxKey, scope)))
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "gtmux"})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	b, err := s.deps.AgentsJSON()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("agents error"))
		return
	}
	// A guest sees ONLY the panes on ITS OWN link's view allowlist (pair-share-
	// model); a full caller (master / paired device) gets the byte-identical radar.
	if callerScope(r.Context()) == scopeGuest {
		dev, ok := callerDevice(r.Context())
		if !ok {
			dev = EnrolledDevice{} // no resolvable link → sees nothing
		}
		b = filterAgentsForGuest(b, dev, s.deps.Share != nil && s.deps.Share.GrantsStale())
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// filterAgentsForGuest drops every agent row whose pane is not on THE CALLER
// LINK's view allowlist (pair-share-model), preserving each surviving row
// byte-for-byte. An empty scope yields an empty radar — a guest can never see
// more than nothing.
func filterAgentsForGuest(raw []byte, dev EnrolledDevice, stale bool) []byte {
	// Grants made against a PREVIOUS tmux server address pane ids that have since been
	// reassigned — showing anything risks revealing a pane the owner never shared.
	if stale {
		return []byte("[]")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return []byte("[]")
	}
	kept := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		var id struct {
			PaneID string `json:"pane_id"`
		}
		if json.Unmarshal(row, &id) == nil && id.PaneID != "" && dev.MayView(id.PaneID) {
			kept = append(kept, row)
		}
	}
	out, err := json.Marshal(kept)
	if err != nil {
		return []byte("[]")
	}
	return out
}

// handleUsage serves the usage-watch report (GET /api/usage), byte-identical to
// `gtmux usage --json`.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	// Usage exposes the whole fleet + token budgets — an owner/HQ surface, not part
	// of a scoped guest's shared view.
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	if s.deps.UsageJSON == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("usage unavailable"))
		return
	}
	b, err := s.deps.UsageJSON()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("usage error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// handleDigest serves the agent-digest array (GET /api/digest) — the supervisor's
// fleet view, byte-identical to `gtmux digest --json`.
func (s *Server) handleDigest(w http.ResponseWriter, r *http.Request) {
	// The digest is the supervisor's whole-fleet view — an owner/HQ surface, never
	// exposed to a scoped guest.
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	if s.deps.DigestJSON == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("digest unavailable"))
		return
	}
	b, err := s.deps.DigestJSON()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("digest error"))
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
	// A guest may read a pane's screen ONLY if it is on ITS OWN link's view list.
	if callerScope(r.Context()) == scopeGuest {
		if s.deps.Share != nil && s.deps.Share.GrantsStale() {
			writeJSON(w, http.StatusForbidden, errBody("forbidden: share is stale (tmux restarted) — the owner must re-grant"))
			return
		}
		if dev, ok := callerDevice(r.Context()); !ok || !dev.MayView(id) {
			writeJSON(w, http.StatusForbidden, errBody("forbidden: pane not shared"))
			return
		}
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
	// Guest gate: a share-link caller may type ONLY when the host consented AND the
	// pane is on THAT LINK's input allowlist (pair-share-model). The owner
	// (master/device) is unrestricted. This is the authoritative server-side check —
	// the web UI only mirrors it.
	if callerScope(r.Context()) == scopeGuest {
		dev, ok := callerDevice(r.Context())
		if !ok || s.deps.Share == nil || !s.deps.Share.InputEnabled() ||
			s.deps.Share.GrantsStale() || !dev.MayInput(req.ID) {
			writeJSON(w, http.StatusForbidden, errBody("input not shared for this pane"))
			return
		}
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
