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
	if !sessionpolicy.ValidHome(r.URL.Query().Get("home_id")) {
		writeJSON(w, 422, errBody("conversation home unavailable"))
		return
	}
	id := r.URL.Query().Get("session_id")
	if id == "" || len(id) > 256 || strings.ContainsAny(id, `*/?[]\`) {
		writeJSON(w, 400, errBody("valid session_id required"))
		return
	}
	setAccessSession(r, id)
	if s.deps.SessionTranscript == nil && s.deps.SessionTranscriptConditional == nil {
		writeJSON(w, 503, errBody("desktop transcript not available"))
		return
	}
	includeContext := callerScope(r.Context()) != scopeGuest && r.URL.Query().Get("context") == "1"
	ifNoneMatch := ""
	if r.URL.Query().Get("before") == "" {
		ifNoneMatch = r.Header.Get("If-None-Match")
	}
	var b []byte
	var meta TranscriptMeta
	var err error
	if s.deps.SessionTranscriptConditional != nil {
		b, meta, err = s.deps.SessionTranscriptConditional(id, r.URL.Query().Get("home_id"), ifNoneMatch, includeContext)
	} else {
		b, meta, err = s.deps.SessionTranscript(id, r.URL.Query().Get("home_id"))
	}
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
