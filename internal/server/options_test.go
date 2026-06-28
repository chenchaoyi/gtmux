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
