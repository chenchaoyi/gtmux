package server

import (
	"net/http"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func (s *Server) handleToolInput(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, 403, errBody("not allowed for a guest connection"))
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, errBody("GET required"))
		return
	}
	q := r.URL.Query()
	sid, home, ref := q.Get("session_id"), q.Get("home_id"), q.Get("ref")
	if sid == "" || len(sid) > 256 || strings.ContainsAny(sid, `*/?[]\`) || !sessionpolicy.ValidRequest(sid, home) || len(ref) > 160 || ref == "" {
		writeJSON(w, 422, errBody("tool input unavailable"))
		return
	}
	setAccessSession(r, sid)
	if s.deps.ToolInput == nil {
		writeJSON(w, 503, errBody("full tool input not supported"))
		return
	}
	data, err := s.deps.ToolInput(sid, home, ref)
	if err != nil {
		writeJSON(w, 404, errBody("tool input unavailable; reload the conversation"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
