package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

type SessionCreated struct {
	Session string `json:"session"`
	PaneID  string `json:"pane_id"`
	Window  string `json:"window"`
	Pane    string `json:"pane"`
	Loc     string `json:"loc"`
}

type SessionCreateError struct{ Code string }

func (e *SessionCreateError) Error() string { return e.Code }

var sessionRequestID = regexp.MustCompile(`^[A-Za-z0-9-]{16,80}$`)

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, code string) { writeJSON(w, status, map[string]string{"error": code, "code": code}) }
	if r.Method != http.MethodPost {
		fail(http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	if callerScope(r.Context()) == scopeGuest {
		lg.Act("act.new", actorOf(r.Context()), "", diag.Refused, "creating a session was refused for a guest")
		fail(http.StatusForbidden, "owner_only")
		return
	}
	if s.deps.CreateSession == nil {
		fail(http.StatusNotImplemented, "unsupported")
		return
	}
	var req struct {
		Name      string `json:"name"`
		RequestID string `json:"request_id"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.Decode(new(any)) != io.EOF || !sessionRequestID.MatchString(req.RequestID) {
		fail(http.StatusBadRequest, "invalid_request")
		return
	}
	if len(req.Name) > 256 || strings.IndexFunc(req.Name, unicode.IsControl) >= 0 {
		fail(http.StatusBadRequest, "invalid_name")
		return
	}
	result, err := s.deps.CreateSession(req.Name, req.RequestID)
	if err != nil {
		lg.Act("act.new", actorOf(r.Context()), "", diag.Failed, "session creation did not return a receipt", "request_id", req.RequestID, "error", err)
		var known *SessionCreateError
		if errors.As(err, &known) {
			fail(http.StatusConflict, known.Code)
			return
		}
		fail(http.StatusServiceUnavailable, "create_failed")
		return
	}
	lg.Act("act.new", actorOf(r.Context()), result.PaneID, diag.OK, "created or recovered a remote session", "request_id", req.RequestID, "session", result.Session, "via", via(r))
	writeJSON(w, http.StatusOK, result)
}
