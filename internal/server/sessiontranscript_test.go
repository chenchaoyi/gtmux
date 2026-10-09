package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestDesktopTranscriptOwnerScopeAndConditionalContract(t *testing.T) {
	calls := 0
	s := New(Config{Token: "master"}, Deps{SessionTranscript: func(id string) ([]byte, TranscriptMeta, error) {
		calls++
		if id == "unknown" {
			return nil, TranscriptMeta{}, sessionpolicy.ErrUnverified
		}
		if id == "failed" {
			return nil, TranscriptMeta{}, errors.New("private path must not leak")
		}
		return []byte(`[{"prompt":"hello","response":"commentary"}]`), TranscriptMeta{Etag: `W/"one"`, Dropped: 3}, nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("guest", []string{"%1"}, nil, 0)
	request := func(method, id, token, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/session/transcript?session_id="+id, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "desk", guest.Token, ""); w.Code != 403 || calls != 0 {
		t.Fatalf("guest reached reader: %d %d", w.Code, calls)
	}
	owner, ok := en.Redeem(en.Mint(), "phone")
	if !ok {
		t.Fatal("fixture")
	}
	for _, token := range []string{"master", owner.Token} {
		w := request("GET", "desk", token, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "commentary") || w.Header().Get("X-Gtmux-Turns-Dropped") != "3" {
			t.Fatal(w)
		}
		w = request("GET", "desk", token, `W/"one"`)
		if w.Code != 304 || w.Body.Len() != 0 || w.Header().Get("X-Gtmux-Turns-Dropped") != "3" {
			t.Fatal(w)
		}
	}
	for _, tc := range []struct {
		method, id string
		want       int
	}{{"POST", "desk", 405}, {"GET", "", 400}, {"GET", "..%2Fdesk", 400}, {"GET", "unknown", 422}, {"GET", "failed", 500}} {
		if w := request(tc.method, tc.id, "master", ""); w.Code != tc.want || strings.Contains(w.Body.String(), "private path") {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
	}
	s.deps.SessionTranscript = nil
	if w := request("GET", "desk", "master", ""); w.Code != 503 {
		t.Fatal(w)
	}
}
