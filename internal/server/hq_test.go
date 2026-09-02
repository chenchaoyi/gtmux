package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	s := hqTestServer(t, Deps{HQEvents: func(sev string, limit int, _ bool) ([]byte, error) {
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
		HQEvents: func(string, int, bool) ([]byte, error) { return []byte(`[{"event":"Waiting"}]`), nil },
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
			s := hqTestServer(t, Deps{Transcript: func(string) ([]byte, TranscriptMeta, error) {
				return body, TranscriptMeta{Dropped: d}, nil
			}})
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

// A conversation that was STARTED OVER must be announced the same way a truncated one
// is. The turns served are complete for the session and still not the whole story —
// what came before is in a previous session log this endpoint never reads. Unannounced,
// a cleared HQ shift reads as a broken app: on 2026-08-09 one showed three bubbles and
// cost a diagnosis (chat-transcript).
func TestTranscriptAnnouncesASessionReset(t *testing.T) {
	body := []byte(`[{"prompt":"hi","response":"yo"}]`)
	for name, tc := range map[string]struct {
		meta     TranscriptMeta
		wantKind string
		wantAt   string
	}{
		"cleared":       {meta: TranscriptMeta{Reset: "clear", ResetAt: 1786253049}, wantKind: "clear", wantAt: "1786253049"},
		"new":           {meta: TranscriptMeta{Reset: "new", ResetAt: 1786253049}, wantKind: "new", wantAt: "1786253049"},
		"clock unknown": {meta: TranscriptMeta{Reset: "clear"}, wantKind: "clear", wantAt: ""}, // kind alone is still worth saying
		"ordinary":      {meta: TranscriptMeta{}, wantKind: "", wantAt: ""},                    // no reset → no headers at all
	} {
		t.Run(name, func(t *testing.T) {
			m := tc.meta
			s := hqTestServer(t, Deps{Transcript: func(string) ([]byte, TranscriptMeta, error) {
				return body, m, nil
			}})
			w := hqGet(t, s, "/api/transcript?id=%251", "master")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; want 200", w.Code)
			}
			if got := w.Header().Get("X-Gtmux-Session-Reset"); got != tc.wantKind {
				t.Errorf("reset header = %q; want %q", got, tc.wantKind)
			}
			if got := w.Header().Get("X-Gtmux-Session-Reset-At"); got != tc.wantAt {
				t.Errorf("reset-at header = %q; want %q", got, tc.wantAt)
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
		Send: func(_, text, _ string, _ bool, _ string) error {
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

// The supervisor is a META layer: hq-meta-layer took it out of the lockscreen tally and
// the "who's waiting" headline, but left the alert loop firing for it — so HQ still
// PUSHED notifications worded and routed exactly like a worker's, for a session the radar
// deliberately no longer lists as one. Getting a push for something you can't find in the
// list is what made the meta-layer read as broken rather than as a layer.
func TestSupervisorProducesNoWorkerAlerts(t *testing.T) {
	var alerts []Alert
	var cur []AgentStatus
	h := newHub(func() []AgentStatus { return cur }, 0, func(a Alert) { alerts = append(alerts, a) })
	h.renudge = time.Hour

	worker := AgentStatus{PaneID: "%7", Agent: "Claude Code", Loc: "api:0.0", Status: "working"}
	hq := AgentStatus{PaneID: "%5", Agent: "Claude Code", Loc: "HQ:0.0", Status: "working", Role: "supervisor"}

	cur = []AgentStatus{worker, hq}
	h.tick() // baseline snapshot — no alerts on first observation
	worker.Status, hq.Status = "waiting", "waiting"
	cur = []AgentStatus{worker, hq}
	h.tick()

	for _, a := range alerts {
		if a.Pane == "%5" {
			t.Errorf("the supervisor emitted a worker alert: %+v", a)
		}
	}
	if len(alerts) == 0 {
		t.Fatal("the WORKER's alert went missing — the gate must be role-scoped, not a blanket mute")
	}

	// ...and a supervisor finishing a turn must not push a "done" either.
	alerts = nil
	worker.Status, hq.Status = "working", "working"
	cur = []AgentStatus{worker, hq}
	h.tick()
	worker.Status, hq.Status = "idle", "idle"
	cur = []AgentStatus{worker, hq}
	h.tick()
	for _, a := range alerts {
		if a.Pane == "%5" {
			t.Errorf("the supervisor emitted a done alert: %+v", a)
		}
	}
}

// ?acts=1 has to reach the producer, or the phone's acts zone silently gets the whole
// mixed feed — which on a real machine is 3.9 hours of mostly wake plumbing where the
// day held 37 acts.
func TestHQEventsActsParamReachesTheProducer(t *testing.T) {
	var gotActs []bool
	s := hqTestServer(t, Deps{HQEvents: func(_ string, _ int, actsOnly bool) ([]byte, error) {
		gotActs = append(gotActs, actsOnly)
		return []byte(`[]`), nil
	}})
	for _, q := range []string{"?acts=1", "", "?acts=0", "?acts=yes"} {
		hqGet(t, s, "/api/hq/events"+q, "master")
	}
	// Only the exact opt-in narrows the feed: anything else must behave as it always did,
	// so a client that does not know the parameter is unaffected.
	if want := []bool{true, false, false, false}; !equalBools(gotActs, want) {
		t.Errorf("actsOnly per request = %v, want %v", gotActs, want)
	}
}

func equalBools(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
