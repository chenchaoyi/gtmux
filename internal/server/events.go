package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
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

// slowTickInterval paces the resource/limits evaluator (df/ps/memory sampling +
// the resource·warn/limits·warn nudge) — much slower than the SSE tick.
const slowTickInterval = 20 * time.Second

// fastTickInterval paces the HQ nudge drain. A wake queued behind a half-typed HQ
// draft is flushed on the next empty box, and this ticker is its unconditional
// backstop — so the wait is ~3s rather than the slow tick's 20s, which is paced by
// resource sampling the drain has nothing to do with. A quiet queue costs one
// readdir (the callback is cheap-gated), so the fast cadence is nearly free.
const fastTickInterval = 3 * time.Second

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
	Role   string // "supervisor" for the HQ pane, else "" — a meta-layer excluded from the worker tally/headline
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

// TallyItem is one listed session in the Live Activity (a session name + its
// status + when its state started). Since is the epoch the item entered its
// current state; the Live Activity widget renders the relative time LOCALLY from
// it (auto-updating on the lock screen), so a mere clock tick no longer counts as
// a tally change — pushes fire only on substantive changes.
type TallyItem struct {
	Title  string `json:"title"`
	Status string `json:"status"` // waiting | working
	Since  int64  `json:"since"`  // epoch seconds the state started; 0 if unknown
}

// tallyMaxItems is how many sessions the Live Activity lists before "+N more".
const tallyMaxItems = 3

// topTallyItems builds the listed sessions: waiting first, then working, each
// most-recent-first, capped at tallyMaxItems. Returns the items + how many active
// (waiting+working) sessions are NOT shown.
func topTallyItems(waiters, workers []AgentStatus) ([]TallyItem, int) {
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
		items = append(items, TallyItem{Title: title, Status: a.Status, Since: a.Since})
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

// ClientInfo identifies one live remote viewer (an /api/events SSE connection):
// a paired phone (Name from the enroll roster, Kind "phone") or an anonymous
// browser mirror (Kind "browser", Platform inferred from the User-Agent). It is
// surfaced so the Mac can show WHO is connected, not just how many. IP/Platform
// stay best-effort (blank behind a tunnel that hides them).
type ClientInfo struct {
	Name        string `json:"name,omitempty"`     // enrolled phone name; "" for a browser
	Kind        string `json:"kind"`               // "phone" | "browser"
	Platform    string `json:"platform,omitempty"` // e.g. "Safari · macOS" (browsers)
	IP          string `json:"ip,omitempty"`       // best-effort remote address (no port)
	ConnectedAt int64  `json:"connectedAt"`        // unix seconds the SSE stream opened
	// DeviceID is the enrolled device's stable id (phones only), used server-side to
	// collapse a device's multiple/overlapping SSE connections into ONE roster row.
	// Not serialized — the roster file only reports the deduped identity.
	DeviceID string `json:"-"`
}

// dedupKey groups a device's connections so the roster shows one row per device:
// a phone by its stable enrolled id (a reconnect behind a tunnel can leave the
// prior connection lingering), a browser by ip+platform.
func (c ClientInfo) dedupKey() string {
	if c.Kind == "phone" && c.DeviceID != "" {
		return "p|" + c.DeviceID
	}
	return c.Kind + "|" + c.Name + "|" + c.Platform + "|" + c.IP
}

// hub fans agent-status changes out to all connected SSE clients. A single
// background loop snapshots agents every eventsInterval, diffs against the
// previous snapshot, and broadcasts: an `agents` event (with a monotonically
// increasing rev) when the set/status changes, and an `alert` event on each
// waiting/done transition. onAlert (optional) lets a push manager hook the same
// transitions without re-deriving them.
type hub struct {
	statuses   func() []AgentStatus
	onAlert    func(Alert)
	onTally    func(Tally)        // fired when the status tally changes (Live Activity push)
	onClients  func([]ClientInfo) // fired with the live remote-viewer roster (who's connected)
	interval   time.Duration
	onSlowTick func()           // resource/limits evaluator, single-goroutine (no nudge race)
	onFastTick func()           // HQ nudge drain, same goroutine — cheap-gated, ~3s
	fastEvery  time.Duration    // onFastTick's cadence (fastTickInterval; tests shorten it)
	renudge    time.Duration    // re-alert a still-waiting pane after this long
	now        func() time.Time // injectable clock (tests)

	mu   sync.Mutex
	subs map[chan sseEvent]ClientInfo
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
		fastEvery:   fastTickInterval,
		renudge:     renudgeInterval,
		now:         time.Now,
		subs:        map[chan sseEvent]ClientInfo{},
		waitAlertAt: map[string]time.Time{},
	}
}

// subscribe registers a new client channel (buffered so a momentarily slow
// reader doesn't block the broadcast). info carries the viewer's identity so the
// roster reflects WHO connected, not just a count.
func (h *hub) subscribe(info ClientInfo) chan sseEvent {
	ch := make(chan sseEvent, 16)
	h.mu.Lock()
	h.subs[ch] = info
	snap := h.snapshotLocked()
	h.mu.Unlock()
	h.emitClients(snap)
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
	snap := h.snapshotLocked()
	h.mu.Unlock()
	h.emitClients(snap)
}

// snapshotLocked returns a stable-sorted copy of the connected clients (oldest
// connection first, then by name) so the roster file doesn't churn on map order.
// Caller must hold h.mu.
func (h *hub) snapshotLocked() []ClientInfo {
	// Collapse a device's multiple/overlapping connections into one entry — a phone
	// behind a tunnel reconnects (network blips / app resumes) and its prior SSE
	// connection can linger, so without this the same phone shows up several times.
	// Keep the most-recent connection's timestamp.
	best := map[string]ClientInfo{}
	for _, info := range h.subs {
		k := info.dedupKey()
		if cur, ok := best[k]; !ok || info.ConnectedAt > cur.ConnectedAt {
			best[k] = info
		}
	}
	out := make([]ClientInfo, 0, len(best))
	for _, info := range best {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ConnectedAt != out[j].ConnectedAt {
			return out[i].ConnectedAt < out[j].ConnectedAt
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// emitClients reports the live remote-viewer roster to the app so it can surface
// WHO is connected. Called outside h.mu.
func (h *hub) emitClients(list []ClientInfo) {
	if h.onClients != nil {
		h.onClients(list)
	}
}

// clientsSnapshot returns the current remote-viewer roster (locking wrapper).
func (h *hub) clientsSnapshot() []ClientInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotLocked()
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
		// The supervisor (HQ) is a META layer — it never counts toward the WORKER
		// fleet tally the lockscreen shows, and never sets the "who's waiting"
		// headline (that headline is about the workers). Change-detection + alerts
		// below still run for it, so the app's HQ card stays live.
		if a.Role != "supervisor" {
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
	// first) so the Live Activity shows concrete session names + a relative time
	// (rendered locally by the widget from each item's Since).
	tally.Items, tally.More = topTallyItems(waiters, workers)

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
	if snap := h.clientsSnapshot(); len(snap) > 0 {
		h.emitClients(snap)
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
	slow := time.NewTicker(slowTickInterval)
	fast := time.NewTicker(h.fastEvery)
	defer t.Stop()
	defer ping.Stop()
	defer slow.Stop()
	defer fast.Stop()
	if h.onSlowTick != nil {
		h.onSlowTick() // an initial evaluation right away
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.tick()
		case <-ping.C:
			h.broadcast(sseEvent{name: "ping", data: []byte("{}")})
		case <-slow.C:
			if h.onSlowTick != nil {
				h.onSlowTick()
			}
		case <-fast.C:
			if h.onFastTick != nil {
				h.onFastTick()
			}
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

	ch := s.hub.subscribe(s.clientInfo(r))
	defer s.hub.unsubscribe(ch)

	// A guest is a SCOPED viewer, not the owner's notification surface: `alert`
	// events carry a pane's session/agent/task, which would leak sessions outside
	// the guest's view allowlist. Guests still get `agents` (a rev bump → they
	// re-fetch the FILTERED /api/agents) and `ping`.
	guest := callerScope(r.Context()) == scopeGuest

	// Immediately sync the just-connected client to the current revision so it
	// fetches /api/agents without waiting for the next change.
	if writeSSE(w, agentsEvent(s.hub.currentRev())) != nil {
		return
	}
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
			if guest && ev.name == "alert" {
				continue // don't leak non-viewable sessions to a guest
			}
			// A failed write means the client is gone even if the context wasn't
			// cancelled (proxy/tunnel keeps the upstream open) — return so the
			// deferred unsubscribe reaps this connection instead of it lingering as
			// a duplicate in the roster. The 20s ping is what forces this check.
			if writeSSE(w, ev) != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// clientInfo derives a viewer's identity from the (already-authenticated)
// request: an enrolled phone (its roster Name) when it presents a device token,
// else an anonymous browser (platform sniffed from the User-Agent). Best-effort —
// a blank name/platform just renders as a generic viewer.
func (s *Server) clientInfo(r *http.Request) ClientInfo {
	info := ClientInfo{Kind: "browser", ConnectedAt: time.Now().Unix(), IP: clientIP(r)}
	if s.deps.Enroll != nil {
		if d, ok := s.deps.Enroll.DeviceByToken(bearerToken(r)); ok {
			info.Kind = "phone"
			info.Name = d.Name
			info.DeviceID = d.ID
			// The phone sends its OS tag (e.g. "iOS 17.5") live per-connection, so
			// it stays correct after an OS update (unlike the enroll-frozen name).
			info.Platform = sanitizeClientTag(r.Header.Get("X-Gtmux-Client"))
			return info
		}
	}
	info.Platform = browserPlatform(r.Header.Get("User-Agent"))
	return info
}

// sanitizeClientTag cleans the phone's self-reported "X-Gtmux-Client" tag before
// it's shown in the Mac's UI: keep only a short, printable "iOS 17.5"-shaped
// string (letters/digits/dot/space), so a hostile client can't inject control
// characters or a wall of text into the roster row.
func sanitizeClientTag(s string) string {
	var b strings.Builder
	for _, r := range s {
		if b.Len() >= 24 {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == ' ':
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// bearerToken extracts the token from an "Authorization: Bearer <t>" header.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, prefix) {
		return strings.TrimPrefix(h, prefix)
	}
	return ""
}

// clientIP returns the viewer's address (no port), best-effort. Behind the
// always-on tunnel RemoteAddr is localhost, so the real client rides in
// X-Forwarded-For (first hop) — but only trust it when it's present.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// browserPlatform sniffs a coarse, human-readable "<Browser> · <OS>" label from a
// User-Agent — enough to tell two browser mirrors apart, not to fingerprint. Order
// matters: Edge/Chrome UAs also contain "Safari"/"Chrome".
func browserPlatform(ua string) string {
	if ua == "" {
		return ""
	}
	browser := ""
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari"):
		browser = "Safari"
	}
	os := ""
	switch {
	case strings.Contains(ua, "iPhone"):
		os = "iPhone"
	case strings.Contains(ua, "iPad"):
		os = "iPad"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	switch {
	case browser != "" && os != "":
		return browser + " · " + os
	case browser != "":
		return browser
	default:
		return os
	}
}

// writeSSE frames one event in the text/event-stream format. Returns the write
// error so the handler can drop a dead connection (the client vanished but its
// context wasn't cancelled — common behind a proxy/tunnel) instead of leaking it.
func writeSSE(w http.ResponseWriter, ev sseEvent) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
	return err
}
