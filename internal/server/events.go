package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	statuses func() []AgentStatus
	onAlert  func(Alert)
	interval time.Duration
	renudge  time.Duration    // re-alert a still-waiting pane after this long
	now      func() time.Time // injectable clock (tests)

	mu   sync.Mutex
	subs map[chan sseEvent]struct{}
	rev  int
	prev map[string]AgentStatus
	// waitAlertAt[pane] is when we last alerted that pane is waiting; used to time
	// re-nudges. Touched only inside tick() (serial), so it needs no extra lock.
	waitAlertAt map[string]time.Time
	started     bool
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
	h.mu.Unlock()
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
	h.mu.Unlock()
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
	for _, a := range cur {
		curMap[a.PaneID] = a
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
