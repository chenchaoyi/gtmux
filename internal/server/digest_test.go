package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GET /api/digest: bearer-gated, serves DigestJSON bytes verbatim, 503 when the
// dep is absent (a serve built without the digest wiring).
func TestDigestEndpoint(t *testing.T) {
	body := `[{"pane_id":"%1","agent":"Claude Code","source":"tmux","status":"waiting","kind":"permission","goal":"fix the login bug","ask":"1.Yes · 2.No"}]`
	s := New(Config{Addr: "x", Token: "master"}, Deps{
		DigestJSON: func() ([]byte, error) { return []byte(body), nil },
	})
	h := s.Handler()

	// guarded
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/digest", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/digest (no token) = %d, want 401", rr.Code)
	}

	// authed → the digest array verbatim
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/digest", nil)
	req.Header.Set("Authorization", "Bearer master")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/digest (authed) = %d, want 200", rr.Code)
	}
	if got := strings.TrimSpace(rr.Body.String()); got != body {
		t.Errorf("digest body = %q, want %q", got, body)
	}

	// nil dep → 503
	s2 := New(Config{Addr: "x", Token: "master"}, Deps{})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/digest", nil)
	req.Header.Set("Authorization", "Bearer master")
	s2.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/digest (no dep) = %d, want 503", rr.Code)
	}
}
