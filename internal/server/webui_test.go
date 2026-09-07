package server

import (
	"bytes"
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
	} else if body := rr.Body.String(); !strings.Contains(body, "openPanes") || !strings.Contains(body, "/api/panes") {
		// the all-panes browser (tiered-pane-control): the radar entry + the fetch
		t.Errorf("app.js missing the all-panes browser wiring (openPanes / /api/panes)")
	}
	// index.html carries the browser's entry button + its own section.
	if rr := get("/"); !strings.Contains(rr.Body.String(), "panes-btn") || !strings.Contains(rr.Body.String(), "id=\"panes\"") {
		t.Errorf("index.html missing the all-panes browser markup (panes-btn / #panes)")
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

// The browser mirror is the one surface you hand to SOMEONE ELSE.
//
// A share link goes to a guest, who is the person least likely to read the host's
// language — and the page had drifted into Chinese-only chrome anyway: the gate screen
// and the connection tooltip were bilingual, the rest was not. This pins that every
// visible label has an English half, so the next label added in one language is a red
// build rather than a page a guest cannot operate.
func TestWebChromeIsNotChineseOnly(t *testing.T) {
	js, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	// Every Chinese string a reader sees must be reachable through the language switch:
	// either T(en, zh), or an explicit ZH ? … : … branch, or the GATE table (which shows
	// BOTH languages on purpose — it is the screen you photograph and send on).
	if !bytes.Contains(js, []byte("function T(en, zh)")) {
		t.Fatal("the language helper is gone; every label would follow whatever the markup happens to say")
	}
	html, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Markup carries the Chinese half; app.js relabels at boot. A control whose id never
	// appears in app.js is one nobody translates.
	for _, id := range []string{"panes-title", "panes-search", "cmdk-input", "jump", "pane-ro", "wb-snap", "wb-surface", "wb-preset"} {
		if !bytes.Contains(html, []byte(`id="`+id+`"`)) {
			continue // the control was removed; nothing to translate
		}
		if !bytes.Contains(js, []byte("'"+id+"'")) {
			t.Errorf("%s is in the page but never relabelled — an English reader sees the Chinese markup", id)
		}
	}
}
