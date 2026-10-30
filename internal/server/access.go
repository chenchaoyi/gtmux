package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

const accessDeviceCtxKey ctxKey = 3
const accessTargetCtxKey ctxKey = 4

type accessTarget struct{ pane, sessionID, outcome string }

type accessReads struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (a *accessReads) keep(key string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if now.Sub(a.last[key]) < 5*time.Minute {
		return false
	}
	if a.last == nil {
		a.last = map[string]time.Time{}
	}
	if len(a.last) >= 1024 {
		for k, at := range a.last {
			if now.Sub(at) >= 5*time.Minute {
				delete(a.last, k)
			}
		}
		// Bound memory without hiding activity: eviction permits extra read records.
		if len(a.last) >= 1024 {
			a.last = map[string]time.Time{}
		}
	}
	a.last[key] = now
	return true
}

var accessRoutes = map[string]string{
	"/api/artifacts/resolve": "document", "/api/artifacts/content": "document",
	"/api/participant-profile": "conversation",
	"/api/agents":              "sessions", "/api/panes": "sessions", "/api/digest": "hq",
	"/api/pane": "terminal", "/api/input-capabilities": "terminal", "/api/transcript": "conversation", "/api/session/transcript": "conversation",
	"/api/diff": "terminal", "/api/usage": "usage", "/api/hq/board": "hq", "/api/hq/workspace": "hq", "/api/hq/events": "hq",
	"/api/hq/knowledge": "knowledge", "/api/hq/knowledge/entry": "knowledge", "/api/hq/memory": "archive_export",
	"/api/push/tokens": "notification_read",
	"/api/host":        "settings", "/api/addresses": "settings", "/api/share": "settings", "/api/share/config": "settings",
	"/api/devices": "settings", "/api/awake": "settings", "/api/routes": "settings", "/api/tasks": "settings",
	"/api/access-history": "history", "/api/options": "terminal", "/api/interaction": "terminal", "/api/session-follow": "settings",
}
var accessWrites = map[string]string{
	"/api/send": "send", "/api/focus": "focus", "/api/sessions": "create_session", "/api/upload": "upload",
	"/api/share/new": "sharing", "/api/share/set": "sharing", "/api/share/config": "sharing", "/api/share/link": "sharing", "/api/share/code": "sharing",
	"/api/enroll/mint": "devices", "/api/devices/rename": "devices", "/api/devices/revoke": "devices",
	"/api/push/register": "notifications", "/api/push/unregister": "notifications", "/api/push/activity": "notifications", "/api/push/test": "notifications", "/api/push/forget": "notifications",
	"/api/routes": "routes", "/api/awake": "server_mode", "/api/hq/knowledge/act": "knowledge_write", "/api/session-follow": "follow",
}

type accessResponse struct {
	http.ResponseWriter
	status int
}

func (w *accessResponse) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *accessResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// Ordinary HTTP responses only. SSE/WebSocket keep their native writer interfaces
// and record connection boundaries at their actual successful-open sites.
func (s *Server) serveAudited(next http.Handler, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/events" || r.URL.Path == "/api/attach" {
		s.serveAuditedStream(next, w, r)
		return
	}
	if r.URL.Path == "/api/icon" || r.URL.Path == "/api/theme" {
		next.ServeHTTP(w, r)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), accessTargetCtxKey, &accessTarget{}))
	started := time.Now()
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	operation := accessRoutes[r.URL.Path]
	if !read && accessWrites[r.URL.Path] != "" {
		operation = accessWrites[r.URL.Path]
	}
	if operation == "" {
		operation = accessWrites[r.URL.Path]
	}
	if operation == "" {
		operation = "other"
	}
	response := &accessResponse{ResponseWriter: w}
	defer func() {
		if failure := recover(); failure != nil {
			s.recordAccess(r, operation, http.StatusInternalServerError, time.Since(started))
			panic(failure)
		}
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		identity := actorOf(r.Context())
		if d, ok := r.Context().Value(accessDeviceCtxKey).(EnrolledDevice); ok {
			identity = d.ID
		} else {
			identity += "/" + clientIP(r)
		}
		origin, _ := accessOrigin(r)
		key := origin + "/" + identity + "/" + operation + "/" + accessPane(r) + "/" + accessSession(r)
		// Full archive exports are security-relevant even when repeated quickly.
		if read && operation != "archive_export" && status < 400 && !accessFailed(r) && !s.accessReads.keep(key, started) {
			return
		}
		s.recordAccess(r, operation, status, time.Since(started))
	}()
	next.ServeHTTP(response, r)
}
func (s *Server) recordAccess(r *http.Request, operation string, status int, duration time.Duration) {
	outcome := diag.OK
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		outcome = diag.Refused
	} else if status >= 400 {
		outcome = diag.Failed
	}
	s.recordAccessOutcome(r, operation, status, duration, outcome)
}
func (s *Server) recordAccessOutcome(r *http.Request, operation string, status int, duration time.Duration, outcome string) {
	if outcome == diag.OK && accessFailed(r) {
		outcome = diag.Failed
	}
	scope := callerScope(r.Context())
	name := "Owner credential"
	var id, platform string
	if d, ok := r.Context().Value(accessDeviceCtxKey).(EnrolledDevice); ok {
		name, id, platform = d.Name, d.ID, d.Platform
	}
	origin, basis := accessOrigin(r)
	lg.Access(diag.AccessEntry{Actor: actorOf(r.Context()), DeviceID: id, Name: name, Platform: platform,
		Origin: origin, OriginBasis: basis,
		SourceIP: clientIP(r), Scope: scope, Operation: operation, Method: r.Method,
		Target: accessPane(r), SessionID: accessSession(r), Status: status, Outcome: outcome, DurationMS: duration.Milliseconds()})
}
func (s *Server) handleAccessHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errBody("method not allowed"))
		return
	}
	if !s.fullOnly(w, r) {
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			writeJSON(w, http.StatusBadRequest, errBody("limit must be 1..200"))
			return
		}
		limit = n
	}
	actor := r.URL.Query().Get("actor")
	if len(actor) > 128 || strings.ContainsAny(actor, "\r\n") {
		writeJSON(w, http.StatusBadRequest, errBody("invalid actor"))
		return
	}
	page, err := diag.ReadAccess(diag.AccessQuery{Before: r.URL.Query().Get("before"), Actor: actor, Limit: limit, Origin: r.URL.Query().Get("origin"), DeviceID: r.URL.Query().Get("device_id"), Operation: r.URL.Query().Get("operation"), Outcome: r.URL.Query().Get("outcome"), GroupBy: r.URL.Query().Get("group_by")})
	if err != nil {
		status := http.StatusServiceUnavailable
		if err == diag.ErrAccessFilter {
			status = http.StatusBadRequest
		}
		if err == diag.ErrAccessCursor {
			status = http.StatusGone
		}
		writeJSON(w, status, errBody("access history unavailable; refresh to start again"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, page)
}

func accessPane(r *http.Request) string {
	id := r.URL.Query().Get("id")
	if target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget); ok && target.pane != "" {
		id = target.pane
	}
	if len(id) < 2 || len(id) > 16 || id[0] != '%' || strings.Trim(id[1:], "0123456789") != "" {
		return ""
	}
	return id
}

func setAccessPane(r *http.Request, pane string) {
	if target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget); ok {
		target.pane = pane
	}
}

func setAccessSession(r *http.Request, id string) {
	if target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget); ok {
		target.sessionID = diag.AccessSessionID(id)
	}
}
func accessSession(r *http.Request) string {
	if target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget); ok {
		return target.sessionID
	}
	return ""
}
func setAccessFailed(r *http.Request) {
	if target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget); ok {
		target.outcome = diag.Failed
	}
}
func accessFailed(r *http.Request) bool {
	target, ok := r.Context().Value(accessTargetCtxKey).(*accessTarget)
	return ok && target.outcome == diag.Failed
}

type accessFlusher struct {
	*accessResponse
	flush http.Flusher
}

func (w accessFlusher) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	w.flush.Flush()
}

type accessHijacker struct {
	*accessResponse
	hijack http.Hijacker
}

func (w accessHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.hijack.Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}
func (s *Server) serveAuditedStream(next http.Handler, w http.ResponseWriter, r *http.Request) {
	response := &accessResponse{ResponseWriter: w}
	var wrapped http.ResponseWriter = response
	operation := "live_open"
	if r.URL.Path == "/api/events" {
		if f, ok := w.(http.Flusher); ok {
			wrapped = accessFlusher{response, f}
		}
	} else {
		operation = "attach_open"
		if h, ok := w.(http.Hijacker); ok {
			wrapped = accessHijacker{response, h}
		}
	}
	defer func() {
		if response.status >= 400 {
			s.recordAccess(r, operation, response.status, 0)
		}
	}()
	next.ServeHTTP(wrapped, r)
}
