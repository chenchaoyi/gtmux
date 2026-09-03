package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// writeRaw sends an already-marshaled JSON body.
func writeRaw(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

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
	// ?acts=1 narrows the feed to what the SUPERVISION did. It is additive: a client that
	// does not send it sees exactly the feed it always saw.
	acts := r.URL.Query().Get("acts") == "1"
	b, err := s.deps.HQEvents(r.URL.Query().Get("severity"), hqEventsLimit(r.URL.Query().Get("limit")), acts)
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

// The knowledge base, remotely (hq-knowledge-on-phone).
//
// Same OWNER rule as the board and the ledger above, for the same reason: the base is the
// supervisor's private assessment of how this machine works, not part of a scoped share.
//
// Reads are split index/detail on size: a few hundred entries with bodies is megabytes
// over a tunnel, and the same identities without them are tens of kilobytes. Writes carry
// exactly two verbs — `land` and `retire` — because those are the two a phone can honestly
// perform in one short line. `add` and `supersede` need prose, and prose typed on a phone
// is how a knowledge base fills with entries nobody wants to read.

// knowledgeAct is the request shape of POST /api/hq/knowledge/act.
type knowledgeAct struct {
	Op  string `json:"op"`  // "land" | "retire"
	ID  string `json:"id"`  // the entry
	Ref string `json:"ref"` // land: where it landed
	Why string `json:"why"` // retire: the reason, which survives
}

// handleHQKnowledge serves the index — every live entry's identity and state, no bodies.
func (s *Server) handleHQKnowledge(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	// No dependency and no knowledge base are the same answer to a client, so neither is
	// an error: it renders "nothing recorded yet" either way.
	if s.deps.HQKnowledge == nil {
		writeRaw(w, []byte(`{"entries":[],"topics":[],"promotions":{"pending":0},"candidates":{"pending":0}}`))
		return
	}
	b, err := s.deps.HQKnowledge()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("knowledge unavailable"))
		return
	}
	writeRaw(w, b)
}

// handleHQKnowledgeEntry serves one entry, with its body.
func (s *Server) handleHQKnowledgeEntry(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errBody("id required"))
		return
	}
	if s.deps.HQKnowledgeEntry == nil {
		writeJSON(w, http.StatusNotFound, errBody("no such entry"))
		return
	}
	b, ok, err := s.deps.HQKnowledgeEntry(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("knowledge unavailable"))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, errBody("no such entry"))
		return
	}
	writeRaw(w, b)
}

// handleHQKnowledgeAct performs one of the two remote mutations.
func (s *Server) handleHQKnowledgeAct(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("POST required"))
		return
	}
	var act knowledgeAct
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&act); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("bad request body"))
		return
	}
	if s.deps.HQKnowledgeAct == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("knowledge unavailable"))
		return
	}
	// The verb list is closed HERE as well as in the dep: a client cannot reach a verb
	// this surface has not decided a phone should have.
	switch act.Op {
	case "land", "retire":
	default:
		writeJSON(w, http.StatusBadRequest, errBody("unknown op (land|retire)"))
		return
	}
	if err := s.deps.HQKnowledgeAct(act.Op, act.ID, act.Ref, act.Why); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
