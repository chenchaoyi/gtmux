package server

import (
	"encoding/json"
	"errors"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"io"
	"net/http"
)

func restoreFailure(w http.ResponseWriter, err error) {
	code := "restore_unavailable"
	status := http.StatusServiceUnavailable
	var known *restorejob.Error
	if errors.As(err, &known) {
		code = known.Code
		switch code {
		case "invalid_request":
			status = 400
		case "not_found":
			status = 404
		case "plan_changed", "request_changed", "request_expired", "restore_busy", "nothing_to_restore", "capacity":
			status = 409
		}
	}
	writeJSON(w, status, map[string]string{"error": code, "code": code})
}
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, 403, map[string]string{"code": "owner_only"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		if s.deps.RestorePreview == nil {
			writeJSON(w, 501, map[string]string{"code": "unsupported"})
			return
		}
		v, e := s.deps.RestorePreview()
		if e != nil {
			restoreFailure(w, e)
			return
		}
		writeJSON(w, 200, v)
	case http.MethodPost:
		if s.deps.RestoreStart == nil {
			writeJSON(w, 501, map[string]string{"code": "unsupported"})
			return
		}
		var req struct {
			ID       string `json:"request_id"`
			Revision string `json:"revision"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF || !restorejob.ValidID(req.ID) || len(req.Revision) != 64 {
			writeJSON(w, 400, map[string]string{"code": "invalid_request"})
			return
		}
		v, e := s.deps.RestoreStart(req.ID, req.Revision)
		if e != nil {
			restoreFailure(w, e)
			return
		}
		diag.For("restore").Act("act.restore", actorOf(r.Context()), "", diag.OK, "restore job reserved or observed", "request_id", req.ID)
		writeJSON(w, 202, v)
	default:
		writeJSON(w, 405, map[string]string{"code": "method_not_allowed"})
	}
}
func (s *Server) handleRestoreJob(w http.ResponseWriter, r *http.Request) {
	if callerScope(r.Context()) == scopeGuest {
		writeJSON(w, 403, map[string]string{"code": "owner_only"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"code": "method_not_allowed"})
		return
	}
	if s.deps.RestoreStatus == nil {
		writeJSON(w, 501, map[string]string{"code": "unsupported"})
		return
	}
	v, e := s.deps.RestoreStatus(r.URL.Query().Get("id"))
	if e != nil {
		restoreFailure(w, e)
		return
	}
	writeJSON(w, 200, v)
}
