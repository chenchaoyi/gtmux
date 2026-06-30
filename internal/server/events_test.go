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

// onClients fires with the live SSE-client count on connect/disconnect, and is
// heartbeated by tick() while clients are connected — the remote-viewer indicator.
func TestHubClientCount(t *testing.T) {
	var counts []int
	h := newHub(func() []AgentStatus { return nil }, time.Hour, nil)
	h.onClients = func(n int) { counts = append(counts, n) }

	a := h.subscribe()
	b := h.subscribe()
	h.tick() // heartbeat while 2 connected → reports 2
	h.unsubscribe(a)
	h.unsubscribe(b)
	h.tick() // none connected → no heartbeat

	want := []int{1, 2, 2, 1, 0}
	if len(counts) != len(want) {
		t.Fatalf("counts = %v, want %v", counts, want)
	}
	for i := range want {
		if counts[i] != want[i] {
			t.Fatalf("counts = %v, want %v", counts, want)
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

// TestHubRenudge verifies a pane that stays waiting re-alerts every renudge
// interval (Repeat=true), and that leaving waiting stops it.
func TestHubRenudge(t *testing.T) {
	working := []AgentStatus{{PaneID: "%1", Agent: "Claude Code", Loc: "a:0.0", Task: "approve?", Status: "working"}}
	waiting := []AgentStatus{{PaneID: "%1", Agent: "Claude Code", Loc: "a:0.0", Task: "approve?", Status: "waiting"}}
	cur := working
	var alerts []Alert
	h := newHub(func() []AgentStatus { return cur }, time.Hour, func(a Alert) { alerts = append(alerts, a) })
	now := time.Unix(0, 0)
	h.now = func() time.Time { return now }
	h.renudge = 5 * time.Minute

	h.tick() // observe working (prev set), no alert
	cur = waiting
	h.tick() // working→waiting: one fresh alert, clock starts at now=0
	if len(alerts) != 1 || alerts[0].Kind != "waiting" || alerts[0].Repeat {
		t.Fatalf("transition: got %+v, want 1 fresh waiting alert", alerts)
	}

	now = now.Add(2 * time.Minute) // inside the interval
	h.tick()
	if len(alerts) != 1 {
		t.Fatalf("re-nudged too early: %+v", alerts)
	}

	now = now.Add(4 * time.Minute) // 6m since the alert → past 5m
	h.tick()
	if len(alerts) != 2 || !alerts[1].Repeat || alerts[1].Pane != "%1" {
		t.Fatalf("expected a re-nudge (Repeat), got %+v", alerts)
	}

	now = now.Add(6 * time.Minute) // past the interval again
	h.tick()
	if len(alerts) != 3 || !alerts[2].Repeat {
		t.Fatalf("expected a second re-nudge, got %+v", alerts)
	}

	cur = working // you acted → no longer waiting
	h.tick()
	now = now.Add(30 * time.Minute)
	h.tick()
	if len(alerts) != 3 {
		t.Fatalf("nudged after leaving waiting: %+v", alerts)
	}
}

func TestHubTally(t *testing.T) {
	cur := []AgentStatus{
		{PaneID: "%1", Agent: "Claude Code", Loc: "proj:0.0", Task: "approve?", Status: "waiting"},
		{PaneID: "%2", Agent: "Codex", Loc: "proj:1.0", Status: "working"},
	}
	var tallies []Tally
	h := newHub(func() []AgentStatus { return cur }, time.Hour, nil)
	h.onTally = func(tl Tally) { tallies = append(tallies, tl) }

	h.tick() // first observation publishes the initial tally
	want0 := Tally{Waiting: 1, Working: 1, WaitingTitle: "approve?", WaitingSession: "proj", Items: []TallyItem{
		{Title: "approve?", Status: "waiting"},
		{Title: "proj", Status: "working"},
	}}
	if len(tallies) != 1 || !tallyEqual(tallies[0], want0) {
		t.Fatalf("initial tally = %+v", tallies)
	}

	h.tick() // unchanged → no new tally push
	if len(tallies) != 1 {
		t.Fatalf("tally pushed without a change: %+v", tallies)
	}

	cur = []AgentStatus{{PaneID: "%2", Agent: "Codex", Loc: "proj:1.0", Status: "idle"}}
	h.tick() // counts changed → push once
	if len(tallies) != 2 || !tallyEqual(tallies[1], Tally{Idle: 1}) {
		t.Fatalf("changed tally = %+v", tallies)
	}
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

func TestRelTime(t *testing.T) {
	cases := []struct {
		now, since int64
		want       string
	}{
		{1000, 0, ""},         // unknown
		{1000, 1000, "now"},   // 0s
		{1000, 970, "now"},    // 30s
		{1000, 880, "2m"},     // 120s
		{10000, 6400, "1h"},   // 3600s
		{200000, 27200, "2d"}, // 172800s
	}
	for _, c := range cases {
		if got := relTime(c.now, c.since); got != c.want {
			t.Errorf("relTime(%d,%d) = %q, want %q", c.now, c.since, got, c.want)
		}
	}
}

func TestTopTallyItems(t *testing.T) {
	now := int64(1000)
	waiters := []AgentStatus{
		{Task: "fix the bug", Status: "waiting", Since: 940}, // 1m
		{Task: "review PR", Status: "waiting", Since: 880},   // 2m
	}
	workers := []AgentStatus{
		{Task: "run tests", Status: "working", Since: 760}, // 4m
		{Task: "build", Status: "working", Since: 700},     // 5m
	}
	items, more := topTallyItems(now, waiters, workers)
	// cap 3: both waiters (newest first) + the newest worker
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	if items[0].Title != "fix the bug" || items[0].Status != "waiting" || items[0].Time != "1m" {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[1].Title != "review PR" || items[1].Time != "2m" {
		t.Errorf("items[1] = %+v", items[1])
	}
	if items[2].Status != "working" || items[2].Time != "4m" { // waiters before workers
		t.Errorf("items[2] = %+v", items[2])
	}
	if more != 1 { // 4 active − 3 shown
		t.Errorf("more = %d, want 1", more)
	}
}

func TestTopTallyItemsTitleFallback(t *testing.T) {
	now := int64(1000)
	// no task → falls back to the session from loc
	items, _ := topTallyItems(now, []AgentStatus{{Loc: "mysess:1.0", Status: "waiting", Since: 900}}, nil)
	if len(items) != 1 || items[0].Title != "mysess" {
		t.Fatalf("title fallback = %+v", items)
	}
}
