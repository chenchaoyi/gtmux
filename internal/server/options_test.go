package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestOptionsEndpoint(t *testing.T) {
	paneText := "Continue?\n ❯ 1. Yes\n   2. No, tell Claude what to do"
	s := New(Config{Addr: "x", Token: "tok"}, Deps{
		PaneText: func(id string) (string, bool) {
			if id == "%1" {
				return paneText, true
			}
			return "", false
		},
	})
	h := s.Handler()

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer tok")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	id1 := "/api/options?id=" + url.QueryEscape("%1")

	// parses the choice block
	rr := get(id1)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var body struct {
		Options []struct {
			N     int    `json:"n"`
			Label string `json:"label"`
		} `json:"options"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Options) != 2 || body.Options[0].N != 1 || body.Options[0].Label != "Yes" {
		t.Fatalf("options = %#v", body.Options)
	}

	// unknown pane → 404
	if get("/api/options?id="+url.QueryEscape("%9")).Code != http.StatusNotFound {
		t.Error("unknown pane should be 404")
	}
	// missing id → 400
	if get("/api/options").Code != http.StatusBadRequest {
		t.Error("missing id should be 400")
	}
	// no token → 401
	noTok := httptest.NewRequest(http.MethodGet, id1, nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, noTok)
	if rr2.Code != http.StatusUnauthorized {
		t.Error("missing token should be 401")
	}
}

// TestOptionsGatedByHookWaiting: when IsWaiting is wired, a pane that is NOT
// hook-waiting returns NO options even if its screen text looks like a menu — the
// approval card must come from the hook, not from parsing arbitrary output.
func TestOptionsGatedByHookWaiting(t *testing.T) {
	// Screen shows a numbered list (like an agent's own "1. … 2. …" prose).
	paneText := "Steps:\n ❯ 1. Yes\n   2. No, tell Claude what to do"
	waiting := map[string]bool{"%waiting": true} // only this pane is hook-waiting
	s := New(Config{Addr: "x", Token: "tok"}, Deps{
		PaneText:  func(string) (string, bool) { return paneText, true },
		IsWaiting: func(id string) bool { return waiting[id] },
	})
	h := s.Handler()
	get := func(id string) []struct {
		N     int    `json:"n"`
		Label string `json:"label"`
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/options?id="+url.QueryEscape(id), nil)
		req.Header.Set("Authorization", "Bearer tok")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		var body struct {
			Options []struct {
				N     int    `json:"n"`
				Label string `json:"label"`
			} `json:"options"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		return body.Options
	}

	// Not hook-waiting → empty, despite the menu-looking screen text.
	if opts := get("%idle"); len(opts) != 0 {
		t.Errorf("non-waiting pane returned options from screen text: %#v", opts)
	}
	// Hook-waiting → the real choices are parsed.
	if opts := get("%waiting"); len(opts) != 2 || opts[0].Label != "Yes" {
		t.Errorf("waiting pane options = %#v, want 2 with first 'Yes'", opts)
	}
}
