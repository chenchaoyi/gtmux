package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// drain non-blockingly collects every event currently buffered on ch.
func drain(ch chan sseEvent) []sseEvent {
	var out []sseEvent
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		default:
			return out
		}
	}
}

func revOf(t *testing.T, ev sseEvent) int {
	t.Helper()
	var v struct {
		Rev int `json:"rev"`
	}
	if err := json.Unmarshal(ev.data, &v); err != nil {
		t.Fatalf("agents data %q: %v", ev.data, err)
	}
	return v.Rev
}

func TestHubDiffAndAlerts(t *testing.T) {
	snaps := [][]AgentStatus{
		{{PaneID: "%1", Agent: "Claude Code", Loc: "a:0.0", Task: "t1", Status: "working"}},
		{{PaneID: "%1", Agent: "Claude Code", Loc: "a:0.0", Task: "t1", Status: "idle"}},    // working→idle = done
		{{PaneID: "%1", Status: "idle"}, {PaneID: "%2", Status: "waiting", Agent: "Codex"}}, // %2 new waiting
		{{PaneID: "%1", Status: "idle"}, {PaneID: "%2", Status: "waiting", Agent: "Codex"}}, // unchanged
	}
	i := 0
	statuses := func() []AgentStatus {
		s := snaps[i]
		if i < len(snaps)-1 {
			i++
		}
		return s
	}
	var alerts []Alert
	h := newHub(statuses, time.Hour, func(a Alert) { alerts = append(alerts, a) })
	ch := h.subscribe()

	h.tick() // first observe → agents rev1, no alert
	h.tick() // %1 working→idle → done alert + agents rev2
	h.tick() // %2 appears waiting → waiting alert + agents rev3
	h.tick() // no change → nothing

	got := drain(ch)
	// Expected interleaving: agents1, alert(done), agents2, alert(waiting), agents3.
	wantNames := []string{"agents", "alert", "agents", "alert", "agents"}
	if len(got) != len(wantNames) {
		t.Fatalf("got %d events %+v, want %d", len(got), names(got), len(wantNames))
	}
	for i, want := range wantNames {
		if got[i].name != want {
			t.Fatalf("event %d = %q, want %q (seq %v)", i, got[i].name, want, names(got))
		}
	}
	if r := revOf(t, got[0]); r != 1 {
		t.Fatalf("first rev = %d, want 1", r)
	}
	if r := revOf(t, got[4]); r != 3 {
		t.Fatalf("last rev = %d, want 3", r)
	}

	if len(alerts) != 2 || alerts[0].Kind != "done" || alerts[0].Pane != "%1" {
		t.Fatalf("alerts[0] = %+v, want done %%1", alerts)
	}
	if alerts[1].Kind != "waiting" || alerts[1].Pane != "%2" || alerts[1].Agent != "Codex" {
		t.Fatalf("alerts[1] = %+v, want waiting %%2 Codex", alerts[1])
	}
}

func names(evs []sseEvent) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.name
	}
	return out
}

// TestHubNilStatuses verifies the loop is a safe no-op without a status source.
func TestHubNilStatuses(t *testing.T) {
	h := newHub(nil, time.Hour, nil)
	ch := h.subscribe()
	h.tick()
	if got := drain(ch); len(got) != 0 {
		t.Fatalf("nil statuses emitted %v", names(got))
	}
	if h.currentRev() != 0 {
		t.Fatalf("rev advanced without statuses")
	}
}

func TestEventsSSEStream(t *testing.T) {
	s := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		AgentStatuses: func() []AgentStatus { return nil },
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// /api/events is auth-guarded like every other /api/* route.
	resp, err := http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	// The handler syncs a just-connected client with an initial agents event.
	got := readEvent(t, bufio.NewReader(resp.Body))
	if !strings.Contains(got, "event: agents") || !strings.Contains(got, `"rev"`) {
		t.Fatalf("initial event = %q", got)
	}
}

// readEvent reads one SSE frame (lines up to the blank separator).
func readEvent(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	var b strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read sse: %v", err)
		}
		if line == "\n" {
			return b.String()
		}
		b.WriteString(line)
	}
}
