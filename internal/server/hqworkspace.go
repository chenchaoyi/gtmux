package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

// Workspace reads share the local CLI's authoritative projection. Reading must
// neither drain instructions nor acknowledge HQ's event watermark.
func (s *Server) handleHQWorkspace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, http.StatusForbidden, errBody("forbidden: not shared"))
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, errBody("GET required"))
		return
	}
	scope := r.URL.Query().Get("matter")
	if len(scope) > 80 || strings.ContainsAny(scope, "/\\\x00") || scope == "global" || scope == "activity" {
		writeJSON(w, http.StatusBadRequest, errBody("invalid matter"))
		return
	}
	var before uint64
	if raw := r.URL.Query().Get("before"); raw != "" {
		var err error
		before, err = strconv.ParseUint(raw, 10, 64)
		if err != nil || scope == "" {
			writeJSON(w, http.StatusBadRequest, errBody("invalid cursor"))
			return
		}
	}
	if s.deps.HQWorkspace == nil {
		writeJSON(w, http.StatusNotImplemented, errBody("workspace unavailable"))
		return
	}
	body, err := s.deps.HQWorkspace(scope, before)
	if errors.Is(err, hqworkspace.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errBody("matter not found"))
		return
	}
	if err != nil || len(body) > 8<<20 {
		writeJSON(w, http.StatusInternalServerError, errBody("workspace unavailable"))
		return
	}
	writeRaw(w, body)
}
