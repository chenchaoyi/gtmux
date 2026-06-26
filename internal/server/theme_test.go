package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/terminal"
)

func TestThemeEndpoint(t *testing.T) {
	theme := terminal.Theme{Source: "ghostty", Background: "#17171a", Foreground: "#d4d2cc", FontFamily: "Hack", FontSize: 15}
	theme.Palette[0] = "#000000"
	s := New(Config{Addr: "x", Token: "master"}, Deps{Theme: func() terminal.Theme { return theme }})
	h := s.Handler()

	// guarded
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/theme", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/theme (no token) = %d, want 401", rr.Code)
	}

	// authed → the resolved theme JSON
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/theme", nil)
	req.Header.Set("Authorization", "Bearer master")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/theme (authed) = %d, want 200", rr.Code)
	}
	var got terminal.Theme
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("theme JSON: %v", err)
	}
	if got.Source != "ghostty" || got.Background != "#17171a" || got.FontFamily != "Hack" {
		t.Errorf("theme = %+v", got)
	}

	// nil dep → 503
	s2 := New(Config{Addr: "x", Token: "master"}, Deps{})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/theme", nil)
	req.Header.Set("Authorization", "Bearer master")
	s2.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("GET /api/theme (no dep) = %d, want 503", rr.Code)
	}
}
