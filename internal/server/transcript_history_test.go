package server

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscriptHistoryKeepsAuthorizationAndContextScope(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("reader", []string{"%1"}, nil, 0)
	calls := 0
	dep := func(string, int) ([]byte, TranscriptMeta, error) {
		return []byte(`[]`), TranscriptMeta{History: func(ctx context.Context, before string) ([]byte, TranscriptMeta, error) {
			calls++
			if before != "older" {
				return nil, TranscriptMeta{}, fmt.Errorf("bad cursor")
			}
			return []byte(`[{"prompt":"public","turn_ordinal":108,"segments":[{"context":{"text":"PRIVATE-CONTEXT","source":"fixture"}}]}]`), TranscriptMeta{}, nil
		}}, nil
	}
	handler := New(Config{Token: "master"}, Deps{Enroll: enroll, Transcript: dep}).Handler()
	for _, tc := range []struct {
		token, id string
		code      int
		private   bool
	}{{"master", "%251", 200, true}, {guest.Token, "%251", 200, false}, {guest.Token, "%252", 403, false}, {"invalid", "%251", 401, false}} {
		r := httptest.NewRequest("GET", "/api/transcript?id="+tc.id+"&context=1&before=older", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.code == 200 {
			if w.Header().Get("X-Gtmux-History-Page") != "1" || strings.Contains(w.Body.String(), "PRIVATE-CONTEXT") != tc.private || !strings.Contains(w.Body.String(), `"turn_ordinal":108`) {
				t.Fatal("page scope", w)
			}
		}
	}
	if calls != 2 {
		t.Fatal("unauthorized history read", calls)
	}
	r := httptest.NewRequest("GET", "/api/transcript?id=%251&before=invalid", nil)
	r.Header.Set("Authorization", "Bearer master")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("stale cursor must be explicit", w.Code)
	}
}
