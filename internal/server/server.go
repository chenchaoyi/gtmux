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
	"net/http"
	"strings"
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

	// Focus selects a pane locally — the "back at your desk, you're already on
	// it" action. It injects no input. err is non-nil if the pane is gone.
	Focus func(id string) error

	// Send types into a pane (WRITE). Exactly one of text/key is used: a non-empty
	// key sends that NAMED key (validated against an allowlist by the impl); else
	// text is typed literally, plus Enter when enter is true. err if the pane is
	// gone or the key isn't allowed. Optional: nil → POST /api/send returns 503.
	Send func(id, text, key string, enter bool) error

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
	return &Server{
		cfg:  cfg,
		deps: deps,
		hub:  newHub(deps.AgentStatuses, eventsInterval, onAlert),
	}
}

// Handler builds the routed, token-guarded http.Handler (exposed for tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth) // unauthenticated probe
	mux.Handle("/api/agents", s.auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("/api/pane", s.auth(http.HandlerFunc(s.handlePane)))
	mux.Handle("/api/focus", s.auth(http.HandlerFunc(s.handleFocus)))
	mux.Handle("/api/send", s.auth(http.HandlerFunc(s.handleSend)))
	mux.Handle("/api/events", s.auth(http.HandlerFunc(s.handleEvents)))
	mux.Handle("/api/push/register", s.auth(http.HandlerFunc(s.handleRegister)))
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
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.Token)) != 1 {
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
	ID   string `json:"id"`
	Text string `json:"text"`
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
	writeJSON(w, http.StatusOK, paneResponse{ID: id, Text: text})
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

// writeJSON writes v as JSON with the given status. Best-effort: a write error
// after the header is sent can't be reported to the client.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
