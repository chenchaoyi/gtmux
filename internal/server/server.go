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
//
// Security model (MVP): every /api/* route (except health) requires a Bearer
// token, compared in constant time. The server is meant to sit behind a
// VPN/tunnel and bind an intranet/VPN interface — never the public internet.
// There is no endpoint that writes to a terminal or runs a command.
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

	// AgentStatuses returns a lean snapshot of current agents for the SSE loop
	// to diff (status transitions → `alert` events + push). Optional: if nil,
	// GET /api/events still serves heartbeats but emits no agents/alert events.
	AgentStatuses func() []AgentStatus

	// OnAlert, if set, is called for every waiting/done transition the events
	// loop detects — the hook a push manager uses to forward alerts to the relay
	// without re-deriving them. Optional.
	OnAlert func(Alert)
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
	return &Server{
		cfg:  cfg,
		deps: deps,
		hub:  newHub(deps.AgentStatuses, eventsInterval, deps.OnAlert),
	}
}

// Handler builds the routed, token-guarded http.Handler (exposed for tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth) // unauthenticated probe
	mux.Handle("/api/agents", s.auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("/api/pane", s.auth(http.HandlerFunc(s.handlePane)))
	mux.Handle("/api/focus", s.auth(http.HandlerFunc(s.handleFocus)))
	mux.Handle("/api/events", s.auth(http.HandlerFunc(s.handleEvents)))
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

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }

// writeJSON writes v as JSON with the given status. Best-effort: a write error
// after the header is sent can't be reported to the client.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
