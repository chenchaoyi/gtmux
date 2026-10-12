package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func (s *Server) handleSessionFollow(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, 403, errBody("not allowed for a guest connection"))
		return
	}
	if s.deps.SessionFollow == nil {
		writeJSON(w, 503, errBody("conversation follow is not available"))
		return
	}
	var id string
	home := r.URL.Query().Get("home_id")
	var next *sessionpolicy.Settings
	switch r.Method {
	case http.MethodGet:
		id = r.URL.Query().Get("session_id")
	case http.MethodPost:
		var body struct {
			HomeID    string                  `json:"home_id,omitempty"`
			SessionID string                  `json:"session_id"`
			Settings  *sessionpolicy.Settings `json:"settings"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || body.Settings == nil {
			writeJSON(w, 400, errBody("session_id and settings are required"))
			return
		}
		if dec.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, errBody("one JSON document required"))
			return
		}
		id, next, home = body.SessionID, body.Settings, body.HomeID
	default:
		writeJSON(w, 405, errBody("GET or POST"))
		return
	}
	if !sessionpolicy.ValidHome(home) {
		writeJSON(w, 422, errBody("Codex home unavailable"))
		return
	}
	if id == "" || len(id) > 256 {
		writeJSON(w, 400, errBody("session_id is required"))
		return
	}
	v, err := s.deps.SessionFollow(id, next, actorOf(r.Context()), home)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, sessionpolicy.ErrConflict) {
			code = http.StatusConflict
		}
		if errors.Is(err, sessionpolicy.ErrUnverified) {
			code = http.StatusUnprocessableEntity
		}
		writeJSON(w, code, errBody(err.Error()))
		return
	}
	writeJSON(w, 200, v)
}
