package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSID005OwnerOptInHistoryAndGuestFence(t *testing.T) {
	calls := 0
	verified := false
	s := New(Config{Token: "master"}, Deps{Transcript: func(string, int) ([]byte, TranscriptMeta, error) {
		if verified {
			return []byte("[]"), TranscriptMeta{Etag: `W/"current"`}, nil
		}
		return nil, TranscriptMeta{}, ErrConversationUnassociated
	}, TranscriptObserved: func(_ string, _ int, limit int) ([]byte, TranscriptMeta, error) {
		calls++
		if limit != 20 {
			t.Errorf("unexpected recent limit %d", limit)
		}
		return []byte(`[{"prompt":"historical"}]`), TranscriptMeta{AssociationReadOnly: true, Etag: `W/"observed"`}, nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("fixture", []string{"%1"}, nil, 0)
	request := func(token, path, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/transcript?id=%251&recent=1"+path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request("master", "", ""); w.Code != 409 || calls != 0 {
		t.Fatal("old client silently received weaker association", w.Code, calls)
	}
	if w := request(guest.Token, "&observation=1", ""); w.Code != 409 || calls != 0 || strings.Contains(w.Body.String(), "historical") {
		t.Fatal("guest reached historical fallback", w.Code, calls)
	}
	if w := request("master", "&observation=1", ""); w.Code != 200 || calls != 1 || w.Header().Get("X-Gtmux-Conversation-State") != "last-known" {
		t.Fatal(w.Code, w.Header())
	}
	if w := request("master", "&observation=1", `W/"observed"`); w.Code != 304 || w.Header().Get("X-Gtmux-Conversation-State") != "last-known" {
		t.Fatal("304 lost restriction", w.Code, w.Header())
	}
	verified = true
	if w := request("master", "&observation=1", `W/"observed"`); w.Code != 200 || w.Header().Get("X-Gtmux-Conversation-State") != "verified" || calls != 2 {
		t.Fatal("verified state did not replace history", w.Code, calls, w.Header())
	}
}

func TestSID005SharedRosterOmitsHistoricalIdentifiers(t *testing.T) {
	raw := []byte(`[{"pane_id":"%1","observed_conversation":{"session_id":"private-history","home_id":"private-home","state":"last-known"}}]`)
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	guest := en.MintGuest("fixture", []string{"%1"}, nil, 0)
	b := filterAgentsForGuest(raw, guest, false)
	var rows []map[string]any
	if json.Unmarshal(b, &rows) != nil || len(rows) != 1 || rows[0]["observed_conversation"] != nil || strings.Contains(string(b), "private") {
		t.Fatal(string(b))
	}
}
