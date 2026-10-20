package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParticipantProfileOwnerGateAndSafeFailures(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	s := New(Config{Token: "owner"}, Deps{ParticipantProfile: func(pane, sid, home, at string) (any, error) {
		calls++
		if sid == "missing" {
			return nil, errors.New("private filesystem path")
		}
		return map[string]any{"name": "Codex", "facts": []any{}, "execution_chain": []any{}}, nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("guest", []string{"%1"}, nil, 0)
	paired, ok := en.Redeem(en.Mint(), "phone")
	if !ok {
		t.Fatal("paired fixture")
	}
	request := func(method, query, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/participant-profile?"+query, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "id=%251", guest.Token); w.Code != 403 || calls != 0 {
		t.Fatal("guest reached private reader", w.Code, calls)
	}
	for _, token := range []string{"owner", paired.Token} {
		w := request("GET", "id=%251", token)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w)
		}
	}
	for _, tc := range []struct {
		method, query string
		want          int
	}{{"POST", "id=%251", 405}, {"GET", "id=not-pane", 400}, {"GET", "session_id=..%2Fprivate", 400}, {"GET", "session_id=s&at=no-clock", 400}, {"GET", "session_id=s&at=1969-01-01T00:00:00Z", 400}, {"GET", "session_id=missing", 422}} {
		w := request(tc.method, tc.query, "owner")
		if w.Code != tc.want || strings.Contains(w.Body.String(), "filesystem") {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
}
