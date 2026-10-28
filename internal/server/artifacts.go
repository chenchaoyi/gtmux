package server

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func artifactIdentity(session, home string) bool {
	return session != "" && len(session) <= 256 && !strings.ContainsAny(session, "\x00\r\n/\\") && sessionpolicy.ValidHome(home)
}

func (s *Server) handleArtifactResolve(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, errBody("POST required"))
		return
	}
	var req struct {
		Reference string `json:"reference"`
		SessionID string `json:"session_id"`
		HomeID    string `json:"home_id"`
		ParentID  string `json:"parent_id,omitempty"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || !artifactIdentity(req.SessionID, req.HomeID) || req.ParentID != "" && (len(req.ParentID) != 32 || strings.Trim(req.ParentID, "0123456789abcdef") != "") || len(req.Reference) > 4096 || strings.ContainsAny(req.Reference, "\x00\r\n") {
		writeJSON(w, 400, errBody("valid document reference required"))
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeJSON(w, 400, errBody("one request required"))
		return
	}
	if req.ParentID == "" && s.deps.ArtifactResolve == nil || req.ParentID != "" && s.deps.ArtifactRelated == nil {
		writeJSON(w, 503, errBody("document previews unavailable"))
		return
	}
	setAccessSession(r, req.SessionID)
	var f artifact.File
	var err error
	if req.ParentID != "" {
		f, err = s.deps.ArtifactRelated(req.Reference, req.ParentID, req.SessionID, req.HomeID)
	} else {
		f, err = s.deps.ArtifactResolve(req.Reference, req.SessionID, req.HomeID)
	}
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "file is unavailable for this conversation", "code": "document_unpublished"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, f)
}

func (s *Server) handleArtifactContent(w http.ResponseWriter, r *http.Request) {
	if !s.fullOnly(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, errBody("GET required"))
		return
	}
	q := r.URL.Query()
	if !artifactIdentity(q.Get("session_id"), q.Get("home_id")) {
		writeJSON(w, 400, errBody("valid conversation required"))
		return
	}
	if s.deps.ArtifactContent == nil {
		writeJSON(w, 503, errBody("document previews unavailable"))
		return
	}
	setAccessSession(r, q.Get("session_id"))
	f, content, err := s.deps.ArtifactContent(q.Get("id"), q.Get("session_id"), q.Get("home_id"))
	if err != nil {
		writeJSON(w, 404, errBody("document unavailable"))
		return
	}
	defer content.Close()
	if f.Bytes <= 0 || f.Bytes > artifact.MaxBytes {
		writeJSON(w, 422, errBody("document exceeds preview limit"))
		return
	}
	// Even HTML is an authenticated download, never active content on serve's
	// origin. Native viewers use the verified metadata after downloading bytes.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Gtmux-SHA256", f.SHA256)
	w.Header().Set("ETag", `"`+f.SHA256+`"`)
	http.ServeContent(w, r, f.Name, time.Time{}, content)
}
