package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The one thing this endpoint must never do is let a remote client enable server
// mode. Everything else here is scope hygiene.

func serverModeTestServer(t *testing.T, offCalled *bool) *Server {
	t.Helper()
	return New(Config{Token: "t0ken"}, Deps{
		ServerModeJSON: func() ([]byte, error) {
			return json.Marshal(map[string]any{"state": "on", "power": "ac"})
		},
		ServerModeOff: func() error {
			if offCalled != nil {
				*offCalled = true
			}
			return nil
		},
	})
}

func TestServerModeRemoteEnableIsRefused(t *testing.T) {
	called := false
	s := serverModeTestServer(t, &called)
	for _, body := range []string{`{"on":true}`, `{}`, ``} {
		req := httptest.NewRequest(http.MethodPost, "/api/awake", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer t0ken")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("POST %q: status %d, want 403 — enabling must never be reachable remotely",
				body, w.Code)
		}
		if !strings.Contains(w.Body.String(), "at the Mac") {
			t.Errorf("POST %q: the refusal must say WHY (it needs authorization at the Mac): %s",
				body, w.Body.String())
		}
	}
	if called {
		t.Error("a refused enable must not have touched the machine")
	}
}

func TestServerModeRemoteOffIsAllowed(t *testing.T) {
	called := false
	s := serverModeTestServer(t, &called)
	req := httptest.NewRequest(http.MethodPost, "/api/awake", strings.NewReader(`{"on":false}`))
	req.Header.Set("Authorization", "Bearer t0ken")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 — de-escalation is the safe direction and must work", w.Code)
	}
	if !called {
		t.Error("turning it off should have reached the machine")
	}
}

func TestServerModeOwnerCanRead(t *testing.T) {
	s := serverModeTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/awake", nil)
	req.Header.Set("Authorization", "Bearer t0ken")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"state"`) {
		t.Errorf("body should carry the status document: %s", w.Body.String())
	}
}

func TestServerModeUnauthenticatedIsRejected(t *testing.T) {
	s := serverModeTestServer(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/awake", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Error("server mode state must not be readable without a token")
	}
}
