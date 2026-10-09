package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func (s *Server) handleSessionTranscript(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, 403, errBody("not allowed for a guest connection"))
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, errBody("GET required"))
		return
	}
	id := r.URL.Query().Get("session_id")
	if id == "" || len(id) > 256 || strings.ContainsAny(id, `*/?[]\`) {
		writeJSON(w, 400, errBody("valid session_id required"))
		return
	}
	if s.deps.SessionTranscript == nil {
		writeJSON(w, 503, errBody("desktop transcript not available"))
		return
	}
	b, meta, err := s.deps.SessionTranscript(id)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, sessionpolicy.ErrUnverified) {
			code = http.StatusUnprocessableEntity
		}
		writeJSON(w, code, errBody("desktop transcript unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeTranscript(w, r, b, meta)
}
