package server

import (
	"net/http"
)

// GET /api/tasks — what was dispatched, and whether it is still running
// (chat-background-tasks).
//
// gtmux has had both halves of this for a long time and never put them together. The
// dispatch ledger knows what was sent, to whom and when; the radar knows whether that
// pane is waiting, working or idle right now. Neither alone answers the question the
// phone actually has, which is "is the thing HQ told me about still going, and does any
// of it need me". The join happens in the app layer, where both are already imported;
// serve only carries the result.
//
// Owner only. A share link invites someone to watch a pane; the ledger says everything
// this machine was told to do, which is a different thing to hand out.

// TaskInfo is one dispatched task with its live state.
type TaskInfo struct {
	ID    string `json:"id"`
	Goal  string `json:"goal"`
	Agent string `json:"agent,omitempty"`
	Pane  string `json:"pane,omitempty"`
	// Status is the PANE's state now: waiting | working | idle, or "gone" when the pane
	// no longer exists. A gone task is still reported: it happened, and a list that
	// silently drops it reads as if it never did.
	Status string `json:"status"`
	// Since is when the task was dispatched (unix seconds).
	Since int64 `json:"since"`
	// Source records which channel created it (hq / user-direct / agent-self).
	Source string `json:"source,omitempty"`
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("not allowed for a guest connection"))
		return
	}
	if s.deps.Tasks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errBody("tasks not available"))
		return
	}
	list, err := s.deps.Tasks()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errBody("could not read the dispatch ledger"))
		return
	}
	if list == nil {
		list = []TaskInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": list})
}
