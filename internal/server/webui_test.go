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

// The browser UI must defeat stale CDN/tunnel edge caching: index.html is served
// uncached with content-hashed asset URLs, and static assets carry no-cache. This
// is what makes `gtmux update` actually show up in the browser.
func TestWebHandlerCacheBusting(t *testing.T) {
	if len(assetTag) != 12 {
		t.Fatalf("assetTag should be a 12-char content hash, got %q", assetTag)
	}

	h := webHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "app.js?v="+assetTag) {
		t.Errorf("index.html app.js not cache-busted with assetTag")
	}
	if !strings.Contains(body, "style.css?v="+assetTag) {
		t.Errorf("index.html style.css not cache-busted with assetTag")
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("index.html Cache-Control missing no-cache: %q", cc)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("GET /app.js = %d", rr2.Code)
	}
	if cc := rr2.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("app.js Cache-Control missing no-cache: %q", cc)
	}
}
