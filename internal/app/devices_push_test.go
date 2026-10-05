package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// gtmux devices --push shows every token the store holds, the paused ones said as such:
// a share link's token, one bound to a device no longer paired (the roster loop alone
// never reached it), and an unattributed one, with how each recovers or is cleared.
func TestListPush_ShowsEveryPausedToken(t *testing.T) {
	t.Setenv("GTMUX_LANG", "en")
	t.Setenv("HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/push/tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{"tokens": []map[string]any{
				{"deviceId": "dev-1", "tokenPrefix": "aaa", "platform": "ios", "env": "sandbox"},
				{"deviceId": "guest-1", "tokenPrefix": "bbb", "platform": "ios", "paused": true},
				{"deviceId": "gone-1", "tokenPrefix": "ccc", "platform": "ios", "paused": true},
				{"tokenPrefix": "ddd", "platform": "ios", "origin": "master"},
				{"tokenPrefix": "eee", "platform": "ios", "paused": true},
			}})
		case "/api/devices":
			_ = json.NewEncoder(w).Encode(map[string]any{"devices": []map[string]any{
				{"id": "dev-1", "name": "phone"}, {"id": "guest-1", "name": "alice"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	out := captureStdout(t, func() {
		if rc := listPush(srv.URL, "tok"); rc != 0 {
			t.Errorf("listPush rc=%d", rc)
		}
	})
	for _, want := range []string{
		"✓ push sandbox", // the owner's device
		"push token paused (a share link is not sent pushes)",
		"1 push token(s) bound to a device no longer paired, paused:",
		"ccc…",
		"1 push token(s) registered with this Mac's own token:",
		"1 unattributed push token(s), paused",
		"eee…",
		"this Mac is reachable",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
