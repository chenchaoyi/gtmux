package server

import (
	"net/http"
	"strconv"
)

// Serving what only the SUPERVISOR knows (hq-command-page).
//
// The radar answers "what is true right now, per session". These two endpoints answer the
// questions it structurally cannot: what does the supervisor THINK is going on (its
// situation board — the one considered synthesis anywhere in the product), and what has
// HAPPENED (the severity-tagged event ledger). Without them a remote HQ page has nothing
// to show but a second copy of the radar, which is exactly the page this change replaces.
//
// Both are OWNER surfaces, refused to a guest for the same reason /api/digest and
// /api/usage are: they carry the whole fleet and the supervisor's private assessment,
// neither of which is part of a scoped share.

// hqEventsDefaultLimit / hqEventsMaxLimit bound the ledger slice. A phone renders a feed,
// not a log file, and an unbounded response over a tunnel is the client's problem to
// download and the server's to marshal.
const (
	hqEventsDefaultLimit = 40
	hqEventsMaxLimit     = 200
)

// hqBoardResponse is the shape of GET /api/hq/board. Exists is explicit so a client can
// say "the supervisor keeps no board yet" instead of guessing from an empty string.
type hqBoardResponse struct {
	Exists    bool   `json:"exists"`
	UpdatedAt int64  `json:"updated_at,omitempty"` // unix secs the board was last written
	Text      string `json:"text,omitempty"`
}

// handleHQBoard serves the supervisor's situation board, read-only.
func (s *Server) handleHQBoard(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	// A missing dependency and a missing board are the same answer to the client, so
	// neither is an error: the page degrades to its deterministic assessment line.
	if s.deps.HQBoard == nil {
		writeJSON(w, http.StatusOK, hqBoardResponse{})
		return
	}
	text, mod, ok := s.deps.HQBoard()
	if !ok {
		writeJSON(w, http.StatusOK, hqBoardResponse{})
		return
	}
	writeJSON(w, http.StatusOK, hqBoardResponse{Exists: true, UpdatedAt: mod, Text: text})
}

// handleHQEvents serves the event ledger at a severity floor, newest first.
func (s *Server) handleHQEvents(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	if s.deps.HQEvents == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
		return
	}
	b, err := s.deps.HQEvents(r.URL.Query().Get("severity"), hqEventsLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("events error"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// hqEventsLimit parses ?limit, clamping to [1, hqEventsMaxLimit]; anything unparseable or
// non-positive falls back to the default rather than being rejected — a feed request with
// a junk limit should still render a feed.
func hqEventsLimit(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return hqEventsDefaultLimit
	}
	if n > hqEventsMaxLimit {
		return hqEventsMaxLimit
	}
	return n
}
