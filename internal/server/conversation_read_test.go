package server

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCAS003ConversationFailureContractAndGuestBoundary(t *testing.T) {
	calls := 0
	s := New(Config{Token: "master"}, Deps{Transcript: func(id string, _ int) ([]byte, TranscriptMeta, error) {
		calls++
		switch id {
		case "%1":
			return nil, TranscriptMeta{}, fmt.Errorf("private-content: %w", ErrConversationUnassociated)
		case "%2":
			return nil, TranscriptMeta{}, ErrConversationUnavailable
		}
		return []byte("[]"), TranscriptMeta{AssociationRecovered: true, Etag: `W/"recovered"`}, nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("fixture", []string{"%3"}, nil, 0)
	request := func(id, token, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/transcript?id="+strings.ReplaceAll(id, "%", "%25"), nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("%1", guest.Token, ""); w.Code != 403 || calls != 0 {
		t.Fatal("guest reached unauthorized reader", w.Code, calls)
	}
	for _, tc := range []struct {
		id     string
		status int
		code   string
	}{{"%1", 409, "conversation-unassociated"}, {"%2", 503, "conversation-unavailable"}} {
		w := request(tc.id, "master", "")
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private-content") {
			t.Fatal(tc, w.Code, w.Body.String())
		}
	}
	for _, etag := range []string{"", `W/"recovered"`} {
		w := request("%3", "master", etag)
		want := 200
		if etag != "" {
			want = 304
		}
		if w.Code != want || w.Header().Get("X-Gtmux-Conversation-State") != "recovered" {
			t.Fatal("recovery status missing", w.Code, w.Header())
		}
	}
}
