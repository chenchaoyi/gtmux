package server

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

var profilePaneID = regexp.MustCompile(`^%[0-9]+$`)

func (s *Server) handleParticipantProfile(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, errBody("GET required"))
		return
	}
	q := r.URL.Query()
	pane, sid, home, at := q.Get("id"), q.Get("session_id"), q.Get("home_id"), q.Get("at")
	if pane != "" && !profilePaneID.MatchString(pane) || len(sid) > 256 || strings.ContainsAny(sid, "*/?[]\\\x00") || !sessionpolicy.ValidHome(home) || pane == "" && sid == "" {
		writeJSON(w, 400, errBody("valid conversation required"))
		return
	}
	if at != "" {
		if tm, e := time.Parse(time.RFC3339Nano, at); e != nil || tm.UnixMilli() <= 0 || sid == "" {
			writeJSON(w, 400, errBody("valid message identity required"))
			return
		}
	}
	if s.deps.ParticipantProfile == nil {
		writeJSON(w, 503, errBody("participant profiles unavailable"))
		return
	}
	setAccessSession(r, sid)
	v, err := s.deps.ParticipantProfile(pane, sid, home, at)
	if err != nil {
		writeJSON(w, 422, errBody("participant identity unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, v)
}
