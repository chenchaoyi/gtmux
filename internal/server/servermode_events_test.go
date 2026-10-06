package server

import (
	"bufio"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The remote-access spec: "A change of server-mode state SHALL be pushed to connected
// clients over the existing live-update channel so a remote surface does not show a
// stale state." The push is a hint to re-read GET /api/awake; it carries no state.

// nextEvent returns the next queued event on ch, or "" when none is queued.
func nextEvent(ch chan sseEvent) string {
	select {
	case ev := <-ch:
		return ev.name
	default:
		return ""
	}
}

func TestAServerModeChangeIsPushedOnce(t *testing.T) {
	sig := ""
	s := New(Config{Token: "t0ken"}, Deps{ServerModeSignature: func() string { return sig }})
	ch := s.hub.subscribe(ClientInfo{})
	defer s.hub.unsubscribe(ch)

	for _, step := range []struct {
		sig, want, why string
	}{
		{"", "", "nothing read yet is not a state"},
		{"off|false", "", "the first reading is the baseline: a client reads the state on connect"},
		{"off|false", "", "an unchanged state is not news"},
		{"on|true", "awake", "a change is pushed"},
		{"", "", "a missing reading is not a change, and keeps the baseline"},
		{"on|true", "", "back to the same state after a missing reading is not a change"},
		{"lapsed|false", "awake", "a lapse is a change too"},
		{"unknown|false", "awake", "and so is a state that cannot be read"},
	} {
		sig = step.sig
		s.hub.checkServerMode()
		if got := nextEvent(ch); got != step.want {
			t.Errorf("sig %q: event %q, want %q (%s)", step.sig, got, step.want, step.why)
		}
	}
}

// An accepted off request tells every owner client to re-read at once; the slow tick
// pushes again when the state actually moves. A refused or failed request pushes nothing.
func TestAnOffRequestTellsOwnersToReRead(t *testing.T) {
	var offErr error
	s := New(Config{Token: "t0ken"}, Deps{
		ServerModeJSON: func() ([]byte, error) { return []byte(`{"state":"on"}`), nil },
		ServerModeOff:  func() error { return offErr },
	})
	ch := s.hub.subscribe(ClientInfo{})
	defer s.hub.unsubscribe(ch)
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/awake", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer t0ken")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		return w.Code
	}

	if code := post(`{"on":true}`); code != http.StatusForbidden || nextEvent(ch) != "" {
		t.Errorf("a refused enable: status %d, and it must push nothing", code)
	}
	offErr = errors.New("marker not written")
	if code := post(`{"on":false}`); code != http.StatusInternalServerError || nextEvent(ch) != "" {
		t.Errorf("a failed off: status %d, and it must push nothing", code)
	}
	offErr = nil
	if code := post(`{"on":false}`); code != http.StatusOK {
		t.Fatalf("off: status %d", code)
	}
	if got := nextEvent(ch); got != "awake" {
		t.Errorf("an accepted off pushed %q, want awake", got)
	}
}

// A guest may not read server mode at all, so its stream never carries the hint. The
// marker event behind it shows the guest's stream is live and simply skipped it.
func TestAGuestStreamNeverCarriesServerMode(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	share := NewShareManager(ShareState{}, nil)
	share.OnBroadcast(enroll.BroadcastGuestScopes)
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Enroll:        enroll,
		Share:         share,
		AgentStatuses: func() []AgentStatus { return nil },
	})
	guest := enroll.MintGuest("g", nil, nil, 0).Token
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	open := func(token string) (*bufio.Reader, func()) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/events", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("events for %q: status %d", token, resp.StatusCode)
		}
		br := bufio.NewReader(resp.Body)
		if first := readEvent(t, br); !strings.Contains(first, "event: agents") {
			t.Fatalf("first event = %q", first)
		}
		return br, func() { resp.Body.Close() }
	}
	owner, closeOwner := open(testToken)
	defer closeOwner()
	g, closeGuest := open(guest)
	defer closeGuest()

	s.hub.broadcast(awakeEvent())
	s.hub.broadcast(agentsEvent(4242))
	if got := readEvent(t, owner); !strings.Contains(got, "event: awake") {
		t.Errorf("the owner's next event = %q, want awake", got)
	}
	if got := readEvent(t, g); strings.Contains(got, "awake") || !strings.Contains(got, `"rev":4242`) {
		t.Errorf("the guest's next event = %q, want the agents marker and no awake", got)
	}
}
