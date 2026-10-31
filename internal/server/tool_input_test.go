package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolInputAuthScopeAndErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	calls := 0
	s := New(Config{Token: "master"}, Deps{ToolInput: func(sid, home, ref string) ([]byte, error) {
		calls++
		if sid == "gone" {
			return nil, errors.New("/private/secret")
		}
		if sid != "session" || home != "" || ref != "reference" {
			t.Fatal(sid, home, ref)
		}
		return []byte(`{"input":"complete","input_ref":"reference"}`), nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("guest", []string{"%1"}, nil, 0)
	owner, ok := en.Redeem(en.Mint(), "phone")
	if !ok {
		t.Fatal("enroll")
	}
	request := func(method, token, query string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/tool-input?"+query, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		method, token, query string
		want                 int
	}{{"GET", "", "session_id=session&ref=reference", 401}, {"GET", guest.Token, "session_id=session&ref=reference", 403}, {"POST", "master", "session_id=session&ref=reference", 405}, {"GET", "master", "session_id=session&home_id=foreign&ref=reference", 422}, {"GET", "master", "session_id=session", 422}} {
		before := calls
		w := request(tc.method, tc.token, tc.query)
		if w.Code != tc.want || calls != before {
			t.Fatal(tc, w.Code, calls, w.Body.String())
		}
	}
	for _, token := range []string{"master", owner.Token} {
		w := request("GET", token, "session_id=session&ref=reference")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), "complete") {
			t.Fatal(w)
		}
	}
	w := request("GET", "master", "session_id=gone&ref=reference")
	if w.Code != 404 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w)
	}
}
