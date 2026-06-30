package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// eventsInterval is how often the hub re-snapshots agents to diff for changes.
// Kept in step with the watch TUI's cadence (internal/app/watch.go watchInterval)
// so the live dashboard and the remote stream observe at the same rate.
const eventsInterval = 1500 * time.Millisecond

// pingInterval is the SSE heartbeat period; it keeps idle connections (and any
// intermediary proxy) from timing the stream out.
const pingInterval = 20 * time.Second

// renudgeInterval is how long a pane may sit `waiting` on the user before the hub
// re-emits its alert (and re-pushes) — so an agent stalled for a decision while
// you're away doesn't go unnoticed. Re-nudges repeat every interval until you act.
const renudgeInterval = 5 * time.Minute

// AgentStatus is the lean per-pane snapshot the events loop diffs — a subset of
// the full `agents --json` contract. The REST GET /api/agents remains the one
// authoritative payload; SSE only signals "something changed, refetch".
type AgentStatus struct {
	PaneID string
	Agent  string
	Loc    string
	Task   string
	Status string // working | waiting | idle | running
	Since  int64  // epoch seconds the current state started (relative-time line); 0 if unknown
}

// Alert is a status transition worth surfacing: a pane that just started
// waiting on the user ("waiting") or finished its turn ("done"). It is both an
// SSE `alert` event (for an in-app banner) and the trigger for a push (later).
type Alert struct {
	Pane   string `json:"pane"`
	Kind   string `json:"kind"` // "waiting" | "done"
	Agent  string `json:"agent"`
	Loc    string `json:"loc"`
	Task   string `json:"task"`
	Repeat bool   `json:"repeat,omitempty"` // a re-nudge: still waiting, not a fresh transition
}

// Tally is the lock-screen Live Activity's content: the per-status counts plus the
// name of the agent that needs you (the headline). Pushed to the activity (via the
// relay) whenever it changes so the lock screen stays current with the app closed.
type Tally struct {
	Waiting        int
	Working        int
	Idle           int
	WaitingTitle   string // the waiting agent's prompt/task (detail line)
	WaitingSession string // the waiting agent's session name (bold headline)
	// Items is the top few in-flight sessions to LIST concretely on the lock screen
	// (waiting first, then working, most-recent first), so the activity shows real
	// session names + a relative time, not just a count. More is how many active
	// sessions are not shown.
	Items []TallyItem
	More  int
}

// TallyItem is one listed session in the Live Activity (a session name + its status
// + a compact relative time like "2m").
type TallyItem struct {
	Title  string `json:"title"`
	Status string `json:"status"` // waiting | working
	Time   string `json:"time"`   // compact relative time, e.g. "now" / "2m" / "1h"
}

// tallyMaxItems is how many sessions the Live Activity lists before "+N more".
const tallyMaxItems = 3

// relTime formats seconds-since into a compact label: "now" (<60s), "Nm" (<1h),
// "Nh" (<1d), else "Nd". Empty when since is unknown (0).
func relTime(nowUnix, since int64) string {
	if since <= 0 {
		return ""
	}
	d := nowUnix - since
	if d < 0 {
		d = 0
	}
	switch {
	case d < 60:
		return "now"
	case d < 3600:
		return itoa(d/60) + "m"
	case d < 86400:
		return itoa(d/3600) + "h"
	default:
		return itoa(d/86400) + "d"
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// topTallyItems builds the listed sessions: waiting first, then working, each
// most-recent-first, capped at tallyMaxItems. Returns the items + how many active
// (waiting+working) sessions are NOT shown.
func topTallyItems(nowUnix int64, waiters, workers []AgentStatus) ([]TallyItem, int) {
	byRecent := func(s []AgentStatus) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Since > s[j].Since }) // newest first
	}
	byRecent(waiters)
	byRecent(workers)
	ordered := append(append([]AgentStatus{}, waiters...), workers...)
	items := make([]TallyItem, 0, tallyMaxItems)
	for _, a := range ordered {
		if len(items) >= tallyMaxItems {
			break
		}
		title := a.Task
		if title == "" {
			title = waitSession(a)
		}
		if title == "" {
			title = a.Agent
		}
		items = append(items, TallyItem{Title: title, Status: a.Status, Time: relTime(nowUnix, a.Since)})
	}
	more := len(waiters) + len(workers) - len(items)
	if more < 0 {
		more = 0
	}
	return items, more
}

// tallyEqual compares two tallies (Tally has a slice, so it isn't `==`-comparable).
func tallyEqual(a, b Tally) bool {
	if a.Waiting != b.Waiting || a.Working != b.Working || a.Idle != b.Idle ||
		a.WaitingTitle != b.WaitingTitle || a.WaitingSession != b.WaitingSession ||
		a.More != b.More || len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if a.Items[i] != b.Items[i] {
			return false
		}
	}
	return true
}

// sseEvent is one named Server-Sent Event ready to frame onto the wire.
type sseEvent struct {
	name string
	data []byte
}

// hub fans agent-status changes out to all connected SSE clients. A single
// background loop snapshots agents every eventsInterval, diffs against the
// previous snapshot, and broadcasts: an `agents` event (with a monotonically
// increasing rev) when the set/status changes, and an `alert` event on each
// waiting/done transition. onAlert (optional) lets a push manager hook the same
// transitions without re-deriving them.
type hub struct {
	statuses  func() []AgentStatus
	onAlert   func(Alert)
	onTally   func(Tally) // fired when the status tally changes (Live Activity push)
	onClients func(int)   // fired with the live SSE-client count (remote-viewer indicator)
	interval  time.Duration
	renudge   time.Duration    // re-alert a still-waiting pane after this long
	now       func() time.Time // injectable clock (tests)

	mu   sync.Mutex
	subs map[chan sseEvent]struct{}
	rev  int
	prev map[string]AgentStatus
	// waitAlertAt[pane] is when we last alerted that pane is waiting; used to time
	// re-nudges. Touched only inside tick() (serial), so it needs no extra lock.
	waitAlertAt map[string]time.Time
	// lastTally / tallyKnown drive Live Activity push-on-change. Touched only in
	// tick() (serial).
	lastTally  Tally
	tallyKnown bool
	started    bool
}

// waitSession is the session name for a waiting agent — the part of loc before
// ':' (e.g. "ccy-workspace" from "ccy-workspace:1.0"), else the whole loc, else
// the agent name. Shown as the Live Activity's bold headline (WHERE to look).
func waitSession(a AgentStatus) string {
	if i := indexByte(a.Loc, ':'); i > 0 {
		return a.Loc[:i]
	}
	if a.Loc != "" {
		return a.Loc
	}
	return a.Agent
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func newHub(statuses func() []AgentStatus, interval time.Duration, onAlert func(Alert)) *hub {
	return &hub{
		statuses:    statuses,
		onAlert:     onAlert,
		interval:    interval,
		renudge:     renudgeInterval,
		now:         time.Now,
		subs:        map[chan sseEvent]struct{}{},
		waitAlertAt: map[string]time.Time{},
	}
}

// subscribe registers a new client channel (buffered so a momentarily slow
// reader doesn't block the broadcast).
func (h *hub) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 16)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	n := len(h.subs)
	h.mu.Unlock()
	h.notifyClients(n)
	return ch
}

// unsubscribe removes and closes a client channel. Safe against a concurrent
// broadcast: both hold h.mu, so no send races the close.
func (h *hub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	n := len(h.subs)
	h.mu.Unlock()
	h.notifyClients(n)
}

// notifyClients reports the live SSE-client count (remote viewers) to the app so
// it can surface a "remote client connected" indicator. Called outside h.mu.
func (h *hub) notifyClients(n int) {
	if h.onClients != nil {
		h.onClients(n)
	}
}

// clientCount returns the number of connected SSE clients.
func (h *hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// currentRev returns the latest agents revision (for a client's initial sync).
func (h *hub) currentRev() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rev
}

// broadcast delivers ev to every subscriber, dropping it for any whose buffer is
// full (a wedged client must never stall the loop or other clients).
func (h *hub) broadcast(ev sseEvent) {
	h.mu.Lock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	h.mu.Unlock()
}

// tick takes one snapshot, diffs it against the previous, and broadcasts the
// resulting events. Exposed (unexported) for deterministic unit tests; the
// background run loop just calls it on a timer.
func (h *hub) tick() {
	if h.statuses == nil {
		return
	}
	cur := h.statuses()
	curMap := make(map[string]AgentStatus, len(cur))
	now := h.now()

	h.mu.Lock()
	prev := h.prev
	h.mu.Unlock()

	changed := prev == nil // first observation always publishes an initial rev
	tally := Tally{}
	var waiters, workers []AgentStatus // collected to LIST the top in-flight sessions
	for _, a := range cur {
		curMap[a.PaneID] = a
		switch a.Status {
		case "waiting":
			tally.Waiting++
			waiters = append(waiters, a)
			if tally.Waiting == 1 { // the first waiter sets the headline
				tally.WaitingTitle = a.Task
				tally.WaitingSession = waitSession(a)
			}
		case "working":
			tally.Working++
			workers = append(workers, a)
		case "idle":
			tally.Idle++
		}
		p, existed := prev[a.PaneID]
		if !existed || p.Status != a.Status || p.Task != a.Task {
			changed = true
		}
		if a.Status == "waiting" {
			al := Alert{Pane: a.PaneID, Kind: "waiting", Agent: a.Agent, Loc: a.Loc, Task: a.Task}
			last, tracked := h.waitAlertAt[a.PaneID]
			switch {
			case prev != nil && p.Status != "waiting":
				// fresh transition into waiting (skip the very first snapshot so a
				// reconnect doesn't replay every agent as a new alert)
				h.emitAlert(al)
				h.waitAlertAt[a.PaneID] = now
			case !tracked:
				// already waiting at first observation — start the clock, don't alert
				h.waitAlertAt[a.PaneID] = now
			case now.Sub(last) >= h.renudge:
				// still waiting after the re-nudge interval → alert again
				al.Repeat = true
				h.emitAlert(al)
				h.waitAlertAt[a.PaneID] = now
			}
		} else {
			delete(h.waitAlertAt, a.PaneID) // no longer waiting → stop tracking
			if prev != nil && a.Status == "idle" && p.Status == "working" {
				h.emitAlert(Alert{Pane: a.PaneID, Kind: "done", Agent: a.Agent, Loc: a.Loc, Task: a.Task})
			}
		}
	}
	for id := range prev { // a pane that disappeared is also a change
		if _, ok := curMap[id]; !ok {
			changed = true
			delete(h.waitAlertAt, id)
		}
	}

	// List the top in-flight sessions (waiting first, then working; most-recent
	// first) so the Live Activity shows concrete session names + a relative time.
	tally.Items, tally.More = topTallyItems(now.Unix(), waiters, workers)

	// Push the Live Activity tally when it changes (counts, headline, or the listed
	// items). Fired outside the lock; the push manager forwards it async to the relay.
	if h.onTally != nil && (!h.tallyKnown || !tallyEqual(tally, h.lastTally)) {
		h.lastTally = tally
		h.tallyKnown = true
		h.onTally(tally)
	}

	// Heartbeat the remote-viewer indicator while clients are connected, so its
	// state file stays fresh and a dead serve (no more ticks) goes stale → the
	// menu bar can treat it as disconnected.
	if n := h.clientCount(); n > 0 {
		h.notifyClients(n)
	}

	h.mu.Lock()
	h.prev = curMap
	if changed {
		h.rev++
		rev := h.rev
		h.mu.Unlock()
		h.broadcast(agentsEvent(rev))
		return
	}
	h.mu.Unlock()
}

// emitAlert broadcasts an SSE alert and invokes the push hook (if any).
func (h *hub) emitAlert(a Alert) {
	if data, err := json.Marshal(a); err == nil {
		h.broadcast(sseEvent{name: "alert", data: data})
	}
	if h.onAlert != nil {
		h.onAlert(a)
	}
}

// agentsEvent frames an `agents` change signal carrying the new revision.
func agentsEvent(rev int) sseEvent {
	return sseEvent{name: "agents", data: []byte(fmt.Sprintf(`{"rev":%d}`, rev))}
}

// run drives tick on h.interval and a heartbeat on pingInterval until ctx is
// done. Started once by the server; a no-op if already running.
func (h *hub) run(ctx context.Context) {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return
	}
	h.started = true
	h.mu.Unlock()

	h.tick() // publish an initial snapshot immediately
	t := time.NewTicker(h.interval)
	ping := time.NewTicker(pingInterval)
	defer t.Stop()
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.tick()
		case <-ping.C:
			h.broadcast(sseEvent{name: "ping", data: []byte("{}")})
		}
	}
}

// handleEvents streams Server-Sent Events: an initial `agents` sync, then live
// `agents`/`alert`/`ping` events until the client disconnects.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errBody("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.hub.subscribe()
	defer s.hub.unsubscribe(ch)

	// Immediately sync the just-connected client to the current revision so it
	// fetches /api/agents without waiting for the next change.
	writeSSE(w, agentsEvent(s.hub.currentRev()))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, ev)
			flusher.Flush()
		}
	}
}

// writeSSE frames one event in the text/event-stream format.
func writeSSE(w http.ResponseWriter, ev sseEvent) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
}
