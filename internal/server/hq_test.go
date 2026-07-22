package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The HQ page's whole reason to exist is data the radar doesn't have. These pin the two
// endpoints that carry it (hq-command-page): owner-only, and "absent" is never an error.

// hqTestServer builds a server whose only credential is the master token.
func hqTestServer(t *testing.T, d Deps) *Server {
	t.Helper()
	return New(Config{Addr: "127.0.0.1:0", Token: "master"}, d)
}

func hqGet(t *testing.T, s *Server, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func TestHQBoardServesTextAndTime(t *testing.T) {
	s := hqTestServer(t, Deps{HQBoard: func() (string, int64, bool) {
		return "# situation\n- niushaofeng waiting", 1750000000, true
	}})
	w := hqGet(t, s, "/api/hq/board", "master")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	var got hqBoardResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Exists || got.UpdatedAt != 1750000000 || got.Text == "" {
		t.Errorf("board = %+v; want exists with text and updated_at", got)
	}
}

// A supervisor that never wrote a board is normal — the page degrades to its
// deterministic line. An error here would put a red state on a healthy setup.
func TestHQBoardAbsentIsNotAnError(t *testing.T) {
	for name, d := range map[string]Deps{
		"no board":      {HQBoard: func() (string, int64, bool) { return "", 0, false }},
		"no dependency": {},
	} {
		t.Run(name, func(t *testing.T) {
			w := hqGet(t, hqTestServer(t, d), "/api/hq/board", "master")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; want 200", w.Code)
			}
			var got hqBoardResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Exists || got.Text != "" {
				t.Errorf("board = %+v; want exists:false and no text", got)
			}
		})
	}
}

func TestHQEventsPassesSeverityAndBoundedLimit(t *testing.T) {
	var gotSev string
	var gotLimit int
	s := hqTestServer(t, Deps{HQEvents: func(sev string, limit int) ([]byte, error) {
		gotSev, gotLimit = sev, limit
		return []byte(`[{"event":"Waiting"}]`), nil
	}})

	if w := hqGet(t, s, "/api/hq/events?severity=notable&limit=5", "master"); w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if gotSev != "notable" || gotLimit != 5 {
		t.Errorf("dep called with (%q, %d); want (notable, 5)", gotSev, gotLimit)
	}

	// A junk or oversized limit must still render a feed, not 400 the client.
	for query, want := range map[string]int{
		"":              hqEventsDefaultLimit,
		"&limit=abc":    hqEventsDefaultLimit,
		"&limit=0":      hqEventsDefaultLimit,
		"&limit=-3":     hqEventsDefaultLimit,
		"&limit=100000": hqEventsMaxLimit,
	} {
		if w := hqGet(t, s, "/api/hq/events?severity=notable"+query, "master"); w.Code != http.StatusOK {
			t.Fatalf("limit %q: status = %d; want 200", query, w.Code)
		}
		if gotLimit != want {
			t.Errorf("limit %q → %d; want %d", query, gotLimit, want)
		}
	}
}

// No ledger dependency yields an empty feed, not a 503: "nothing happened" and "I can't
// tell you" look identical to a reader, and only one of them needs an error state.
func TestHQEventsWithoutDependencyIsEmptyNotError(t *testing.T) {
	w := hqGet(t, hqTestServer(t, Deps{}), "/api/hq/events", "master")
	if w.Code != http.StatusOK || w.Body.String() != "[]" {
		t.Errorf("status=%d body=%q; want 200 []", w.Code, w.Body.String())
	}
}

// Both carry the WHOLE fleet plus the supervisor's private assessment — owner surfaces,
// exactly like /api/digest and /api/usage.
func TestHQSurfacesAreRefusedToAGuest(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("visitor", []string{"%1"}, nil, 0)
	s := hqTestServer(t, Deps{
		Enroll:   enroll,
		HQBoard:  func() (string, int64, bool) { return "secret assessment", 1, true },
		HQEvents: func(string, int) ([]byte, error) { return []byte(`[{"event":"Waiting"}]`), nil },
	})
	for _, p := range []string{"/api/hq/board", "/api/hq/events"} {
		w := hqGet(t, s, p, guest.Token)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s as guest -> %d; want 403", p, w.Code)
		}
		if body := w.Body.String(); strings.Contains(body, "secret") || strings.Contains(body, "Waiting") {
			t.Errorf("%s leaked owner data to a guest: %s", p, body)
		}
	}
}

// A truncated history must be ANNOUNCED, so a client can say "earlier turns not shown"
// instead of presenting part of a conversation as the whole one. It rides a header, not
// an envelope: the body stays the plain turn array older app builds already parse
// (transcript-render-bounds).
func TestTranscriptAnnouncesDroppedTurns(t *testing.T) {
	body := []byte(`[{"prompt":"hi","response":"yo"}]`)
	for name, tc := range map[string]struct {
		dropped    int
		wantHeader string
	}{
		"truncated": {dropped: 112, wantHeader: "112"},
		"whole":     {dropped: 0, wantHeader: ""}, // nothing dropped → no header at all
	} {
		t.Run(name, func(t *testing.T) {
			d := tc.dropped
			s := hqTestServer(t, Deps{Transcript: func(string) ([]byte, int, error) { return body, d, nil }})
			w := hqGet(t, s, "/api/transcript?id=%251", "master")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; want 200", w.Code)
			}
			if got := w.Header().Get("X-Gtmux-Turns-Dropped"); got != tc.wantHeader {
				t.Errorf("dropped header = %q; want %q", got, tc.wantHeader)
			}
			if w.Body.String() != string(body) {
				t.Errorf("body = %s; want the turn array unchanged (no envelope)", w.Body.String())
			}
		})
	}
}

// A send the pane never submitted must be REPORTED. serve used to discard the
// paste-and-submit verdict and return nil, so the API answered success while the
// message sat unsubmitted in the input box — the phone showed it as sent and the only
// way to learn otherwise was to walk over to the Mac.
func TestSendReportsAMessageThatWasNotSubmitted(t *testing.T) {
	s := hqTestServer(t, Deps{
		Send: func(_, text, _ string, _ bool) error {
			if text == "a very long message" {
				return errSendNotSubmitted
			}
			return nil
		},
	})
	post := func(text string) *httptest.ResponseRecorder {
		body := `{"id":"%1","text":"` + text + `","enter":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer master")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		return w
	}
	if w := post("a very long message"); w.Code == http.StatusOK {
		t.Error("an unsubmitted send answered 200 — the client cannot tell it failed")
	} else if !strings.Contains(w.Body.String(), "send failed") {
		t.Errorf("body = %s; want a send-failed error the client can surface", w.Body.String())
	}
	if w := post("ok"); w.Code != http.StatusOK {
		t.Errorf("a normal send = %d; want 200", w.Code)
	}
}

var errSendNotSubmitted = errSend("not submitted: the pane's input box did not settle on the full message")

type errSend string

func (e errSend) Error() string { return string(e) }
