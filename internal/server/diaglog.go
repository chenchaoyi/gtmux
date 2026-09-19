package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// serve records what it saw and what it did (openspec change `diagnostics`). It wrote
// nothing at runtime before this: on 2026-09-19 a phone's pairing failed and there was no
// trace of the request on the Mac that received it.
var lg = diag.For("serve")

const actorCtxKey ctxKey = 2

// actorOf names who is behind an authenticated request, for action entries: "owner" for
// the master token (the menu bar and the terminal share it), the device for a paired
// device's own token, the link for a guest. Unauthenticated requests are "anonymous".
func actorOf(ctx context.Context) string {
	if a, ok := ctx.Value(actorCtxKey).(string); ok && a != "" {
		return a
	}
	return "anonymous"
}

// deviceActor spells a roster entry as an actor: phone:, browser: or device:, then the
// first 8 characters of its id, which is enough to find it in `gtmux devices`.
func deviceActor(d EnrolledDevice) string {
	id := d.ID
	if len(id) > 8 {
		id = id[:8]
	}
	if d.Scope == scopeGuest {
		return "guest:" + id
	}
	p := strings.ToLower(d.Platform + " " + d.Name)
	switch {
	case strings.Contains(p, "ios") || strings.Contains(p, "iphone") || strings.Contains(p, "ipad"):
		return "phone:" + id
	case strings.Contains(p, "safari") || strings.Contains(p, "chrome") || strings.Contains(p, "firefox") ||
		strings.Contains(p, "edge") || strings.Contains(p, "browser"):
		return "browser:" + id
	}
	return "device:" + id
}

// payloadSum is a short hash of what was sent, so an action entry can be matched to the
// journal's receipt without the log ever holding the text.
func payloadSum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}

// rejects aggregates failed authentication: the first one in a minute is written with
// its route, and the rest of that minute become one count when the next minute starts.
// A scanner on the tunnel must not be able to fill the log.
type rejects struct {
	mu     sync.Mutex
	window time.Time
	count  int
}

var authRejects rejects

func (r *rejects) note(req *http.Request) {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Sub(r.window) >= time.Minute {
		if r.count > 1 {
			lg.Warn("auth.rejected.summary", "more requests with an unknown token in the previous minute",
				"count", r.count-1, "since", r.window.Format(time.RFC3339))
		}
		r.window, r.count = now, 0
	}
	r.count++
	if r.count == 1 {
		lg.Warn("auth.rejected", "a request with a missing or unknown token was refused",
			"route", req.URL.Path, "via", via(req))
	}
}

// via says whether a request came through the tunnel or from this Mac or the LAN.
func via(r *http.Request) string {
	if r.Header.Get("Cf-Ray") != "" || r.Header.Get("X-Forwarded-For") != "" {
		return "tunnel"
	}
	return "direct"
}

// recoverPanics turns a handler panic into a 500 and an error entry with the stack's
// top, instead of net/http's line on stderr that nobody reads.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				stack := string(debug.Stack())
				if len(stack) > 1500 {
					stack = stack[:1500]
				}
				lg.Error("handler.panic", "a request handler panicked", "route", r.URL.Path,
					"panic", toString(v), "stack", stack)
				writeJSON(w, http.StatusInternalServerError, errBody("internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case error:
		return t.Error()
	}
	return "panic"
}

// uploadExt is the extension of an uploaded file, the only part of its name the log
// keeps.
func uploadExt(name string) string { return strings.ToLower(filepath.Ext(name)) }
