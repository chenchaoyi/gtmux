package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The browser-mirror web UI is served at "/" (unauthenticated static page), its
// vendored assets resolve, and the /api/* routes stay token-guarded.
func TestWebUIRouting(t *testing.T) {
	s := New(Config{Addr: "x", Token: "master"}, Deps{})
	h := s.Handler()

	get := func(path string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		return rr
	}

	if rr := get("/"); rr.Code != http.StatusOK {
		t.Errorf("GET / = %d, want 200", rr.Code)
	} else if !strings.Contains(rr.Body.String(), "app.js") {
		t.Errorf("GET / did not serve index.html (no app.js reference)")
	}

	if rr := get("/app.js"); rr.Code != http.StatusOK {
		t.Errorf("GET /app.js = %d, want 200", rr.Code)
	}
	if rr := get("/vendor/xterm.js"); rr.Code != http.StatusOK {
		t.Errorf("GET /vendor/xterm.js = %d, want 200", rr.Code)
	}

	// The web "/" route must NOT shadow the guarded API.
	if rr := get("/api/agents"); rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/agents (no token) = %d, want 401", rr.Code)
	}
}
