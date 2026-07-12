package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GET /api/usage: bearer-gated, serves UsageJSON verbatim, 503 without the dep.
func TestUsageEndpoint(t *testing.T) {
	body := `{"sessions":[{"loc":"a:0.0","agent_key":"claude","tok":123,"ctx":0.85,"usage_warn":"ctx 85%"}],"types":[{"agent_key":"claude","sessions":1,"tok":123,"rate":10}]}`
	s := New(Config{Addr: "x", Token: "master"}, Deps{
		UsageJSON: func() ([]byte, error) { return []byte(body), nil },
	})
	h := s.Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/usage", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	req.Header.Set("Authorization", "Bearer master")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != body {
		t.Fatalf("authed = %d %q", rr.Code, rr.Body.String())
	}

	s2 := New(Config{Addr: "x", Token: "master"}, Deps{})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	req.Header.Set("Authorization", "Bearer master")
	s2.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("no dep = %d, want 503", rr.Code)
	}
}
