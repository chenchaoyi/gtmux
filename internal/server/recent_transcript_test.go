package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLCL002RecentAndHistoryReadersAreOptInAndAuthorized(t *testing.T) {
	defaults, recent, pages := 0, 0, 0
	full := func(string, int) ([]byte, TranscriptMeta, error) {
		defaults++
		return []byte(`[{"prompt":"full"}]`), TranscriptMeta{Etag: `W/"full"`}, nil
	}
	window := func(id string, earlier int, historyOnly bool) ([]byte, TranscriptMeta, error) {
		if id != "%1" {
			t.Fatal("unauthorized producer", id)
		}
		if historyOnly {
			return []byte(`[]`), TranscriptMeta{History: func(context.Context, string) ([]byte, TranscriptMeta, error) {
				pages++
				return []byte(`[{"prompt":"older","turn_ordinal":10,"segments":[{"context":{"text":"PRIVATE"}}]}]`), TranscriptMeta{}, nil
			}}, nil
		}
		recent++
		return []byte(`[{"prompt":"recent","turn_ordinal":91}]`), TranscriptMeta{Etag: `W/"recent"`, Dropped: 90}, nil
	}
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("reader", []string{"%1"}, nil, 0)
	handler := New(Config{Token: "master"}, Deps{Enroll: enroll, Transcript: full, TranscriptWindow: window}).Handler()
	for _, tc := range []struct {
		query, token, want string
		code               int
	}{
		{"", "master", "full", 200}, {"&recent=1", "master", "recent", 200}, {"&recent=other", "master", "full", 200},
		{"&recent=1&before=older&context=1", "master", "PRIVATE", 200}, {"&recent=1&before=older&context=1", guest.Token, "older", 200}, {"&recent=1", "invalid", "", 401},
	} {
		req := httptest.NewRequest("GET", "/api/transcript?id=%251"+tc.query, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if tc.token == guest.Token && strings.Contains(w.Body.String(), "PRIVATE") {
			t.Fatal("guest context exposed")
		}
	}
	if defaults != 2 || recent != 1 || pages != 2 {
		t.Fatal("history did a full read or auth bypass", defaults, recent, pages)
	}
	// Old dependency providers remain compatible when no optimized reader exists.
	req := httptest.NewRequest("GET", "/api/transcript?id=%251&recent=1", nil)
	req.Header.Set("Authorization", "Bearer master")
	w := httptest.NewRecorder()
	New(Config{Token: "master"}, Deps{Transcript: full}).Handler().ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "full") {
		t.Fatal("legacy provider incompatible", w)
	}
}
