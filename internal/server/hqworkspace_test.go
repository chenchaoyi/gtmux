package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
)

func TestHQWorkspaceReadBoundary(t *testing.T) {
	calls := 0
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("visitor", []string{"%1"}, nil, 0)
	s := hqTestServer(t, Deps{Enroll: enroll, HQWorkspace: func(matter string, before uint64) ([]byte, error) {
		calls++
		if matter == "missing" {
			return nil, hqworkspace.ErrNotFound
		}
		if matter == "failed" {
			return nil, errors.New("private source path")
		}
		if matter == "large" {
			return make([]byte, 8<<20+1), nil
		}
		if matter == "goal" && before != 42 {
			t.Errorf("cursor = %d", before)
		}
		return []byte(`{"schema":1}`), nil
	}})
	for _, path := range []string{"/api/hq/workspace", "/api/hq/workspace?matter=goal&before=42"} {
		w := hqGet(t, s, path, "master")
		if w.Code != 200 || w.Body.String() != `{"schema":1}` || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w)
		}
	}
	before := calls
	w := hqGet(t, s, "/api/hq/workspace", guest.Token)
	if w.Code != 403 || calls != before {
		t.Fatal("guest reached workspace", w.Code, calls)
	}
	for query, code := range map[string]int{"?matter=global": 400, "?matter=../goal": 400, "?before=42": 400, "?matter=goal&before=oops": 400, "?matter=missing": 404, "?matter=failed": 500, "?matter=large": 500} {
		w := hqGet(t, s, "/api/hq/workspace"+query, "master")
		if w.Code != code || strings.Contains(w.Body.String(), "private source path") {
			t.Fatal(query, w.Code, w.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/hq/workspace", strings.NewReader(`{"op":"send"}`))
	req.Header.Set("Authorization", "Bearer master")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != 405 || w.Header().Get("Allow") != "GET" {
		t.Fatal(w.Code)
	}
	if w := hqGet(t, hqTestServer(t, Deps{}), "/api/hq/workspace", "master"); w.Code != 501 {
		t.Fatal(w.Code)
	}
}
