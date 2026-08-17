// The serve slow-tick evaluator (resource-watch + limits-watch): the SINGLE
// place that samples machine resources + subscription limits and nudges a live
// gtmux HQ on a NEW warning. It runs from the hub's one goroutine (server
// OnSlowTick), so its dedup markers have no read-check-write race — this is the
// fix for the limits·warn 3× bug (its nudge used to live in gatherUsage, which
// /api/usage + the HQ card + the CLI call concurrently).
package hq

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resource"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tabalert"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// SlowTickEval is wired to server Deps.OnSlowTick. It evaluates resource +
// subscription-limit warnings, delivers the HQ summary tick, and nudges HQ once
// per new/changed warning.
func SlowTickEval() {
	DrainHQNudges() // also on the 3s fast tick — this covers serve's first evaluation
	// Resource: sample + nudge on a NEW machine warn TIER. Dedup keys on the tier
	// (amber/red), NOT the exact warn value — disk-free jittering 40→39→38 GB stays
	// amber and must NOT re-nudge per GB (the by-tier fix) — and the tier itself is
	// damped three ways (hysteresis + confirmation + restate interval) so a value
	// dithering ON a threshold can't re-alert either.
	rep := radar.CurrentResource()
	if resourceTierGate(rep.Machine, time.Now().Unix()) {
		nudgeHQPane(hqwake.Line(hqwake.ClassResourceWarn, "", rep.Machine.Warn), orphanTail(rep))
	}
	// Limits: cache-gated refresh (spawns claude at most once per TTL), nudge on
	// a new weekly-window crossing. (Moved here from gatherUsage — the 3× fix.)
	// Dedup keys on the WINDOW identity, so a % climbing within the same window
	// (93→94→95%) doesn't re-nudge. Deliberately NOT tier-gated like resource: a
	// window identity has no severity order, so "less severe" is undefined and a
	// suppressed first warning for a new window would be a loss, not a damped flap.
	lr, _ := limits.Get(limits.LoadConfig(), false, time.Now())
	nudgeOnChange("limitswarn", limitsTierKey(lr.Warn),
		hqwake.Line(hqwake.ClassLimitsWarn, "", lr.Warn), "")
	// Terminal-tab attention marker: the BACKSTOP. The hook updates it instantly on a
	// waiting transition; this catches a pane whose event never landed, a session created
	// since, and a marker left behind by a pane that is gone. No-op when the feature is off.
	tabalert.Reconcile()
	// Lifecycle watchdog (charter M5): escalate a pane stuck waiting past the timeout.
	watchdogSweep(time.Now().Unix())
	// Stuck-dispatch (stuck-dispatch-waiting): a dispatched worker blocked BEFORE running
	// a turn (startup/permission gate, or its goal left unsubmitted) fires no hook, so it
	// would read idle → done. Persist a `waiting` marker + fire one `waiting` wake so HQ
	// unblocks it instead of being told the task finished.
	stuckDispatchSweep()
	// Resolved backstop (hq-send-delivery-reliability B): fire a `resolved` wake when a
	// pane LEAVES the waiting state via a path no hook event covers — a permission/
	// question approved in the source window, after which the agent just resumes (Claude
	// registers no PostToolUse, so its wait marker lingers until the eventual Stop). The
	// hook's resolved emit is event-gated; this observes the radar's status transition.
	resolvedTransitionSweep()
	// Wake-channel watchdog (hq-wake-reliability): the knock itself can break —
	// escalate OUT of band when it does.
	wakeWatchdog(time.Now().Unix())
	// Self-check sensor (hq-attention-system §8): raise a self-check trigger to HQ when
	// due (idle/threshold/daily), rate-limited to ≤ 1/h. No LLM here — HQ does the pass.
	selfCheckSensor(time.Now().Unix())
	// Distill sensor (hq-knowledge-distillation): raise a periodic knowledge-distillation
	// trigger when due (weekly floor / event-volume floor, zero-change gated, ≤ 1/day).
	// No LLM here — HQ distils the fleet delta into the KB and prunes stale.
	distillSensor(time.Now().Unix())
	// Summary tick (hq-perception-v2): deliver the periodic brief wake ONLY when
	// outcome-level changes accumulated (the zero-change gate — a quiet interval
	// injects nothing and costs no tokens).
	hqSummaryTick(time.Now().Unix())
	// Completeness net (hq-watermark-wakes): knock when the stream has moved past HQ's
	// consumption watermark and stayed there. Deliberately LAST of the wake-raising
	// sensors — everything above has already had its chance to knock about the specific
	// events it recognizes, so this only ever speaks about what none of them claimed.
	unreadSensor(time.Now().Unix())
	// HQ session health (hq-self-rotate): the one sensor whose subject is the supervisor
	// itself — is this session still fit to judge, or is it long/full enough that its own
	// output starts reading back as input? HQ cannot self-detect that (the failing faculty
	// is the one that would have to notice) and cannot self-schedule it (no timer between
	// wakes), so the resident tick asks on its behalf and HQ does the rotation.
	selfRotateSensor(time.Now().Unix())
	// Disk hygiene (resource-watch): cap the never-rotated launchd logs + prune the
	// uploads sink so gtmux can't fill the disk with its own output. Time-gated to
	// ≤ 1/30 min; silent housekeeping (no HQ nudge).
	diskHygieneSweep(time.Now().Unix())
}

// hqSummaryTick delivers the `tick` wake when due: at least one pending outcome
// AND (interval elapsed OR burst threshold). Gated on a live HQ; the tally keeps
// accumulating when HQ is away and the first tick after it returns covers it all.
func hqSummaryTick(now int64) {
	if !hqwake.TickDue(now, hqwake.Load()) {
		return
	}
	pane := hqpane.Find()
	if pane == "" {
		return // no HQ to wake — leave the tally accumulating
	}
	if line := hqwake.ConsumeTick(now, events.LatestSeq()); line != "" {
		hqnudge.Deliver(pane, line)
	}
}

// DrainHQNudges flushes any HQ nudges queued behind a half-typed draft (or a pane in
// copy-mode, or a moment with no resolvable HQ). Wired to the 3s fast tick: a knock
// that arrives while the user is typing must land in seconds, not wait on the
// resource-sampling cadence. Cheap-gated on Pending() — a dir scan — so a quiet queue
// costs no tmux at all.
func DrainHQNudges() {
	if hqnudge.Pending() {
		hqnudge.Drain(hqpane.Find())
	}
}

// journalDegradation appends one perception-degradation control record to the
// session JOURNAL — the carrier the pull side actually reads. These records used
// to go through hqsurface.EmitControl, which writes ONLY to the feed spool: neither
// reader could receive them there (the playbook tells HQ not to tail the feed,
// and `gtmux events` reads the journal), so 75 degradation records accumulated
// invisible to every query — the same #647 failure shape the maintenance
// triggers already escaped. NOT an audit record: a degradation is new
// information for HQ, so it counts toward the consumption debt and reaches HQ
// through the completeness net even when the wake channel is the casualty.
func journalDegradation(event, summary string, now int64) {
	events.Append(events.Record{
		Ts: now, Event: event, Summary: summary, Severity: events.SevImportant,
	})
}

// wakeWatchdog surfaces a wake channel that is failing to reach a LIVE HQ (no HQ is
// not a degradation — there is simply nothing to wake). It runs from the single-writer
// slow tick and escalates once per transition into degraded.
//
// The alarm cannot ride the channel it is about, so it takes three carriers: a journal
// control record at important severity (the pull side sees it), a best-effort HQ line
// (free, and it lands whenever only the ACK was flaky), and a desktop notification —
// the one carrier that does not depend on the broken thing. A perception outage must
// never stay silent.
func wakeWatchdog(now int64) {
	key := ""
	if hqpane.Find() != "" && hqnudge.Degraded(now) {
		key = "down"
	}
	if !markerChanged("hqwakedegraded", key) {
		return // unchanged (or recovered — which clears the marker and stays quiet)
	}
	const summary = "⚠ HQ wake channel not landing — knocks are queued but unconfirmed; " +
		"reconcile by pull: gtmux events --since-seq <n>"
	journalDegradation(hqsurface.ControlWakeDegraded, summary, now)
	if pane := hqpane.Find(); pane != "" {
		hqnudge.Deliver(pane, hqwake.Line(hqwake.ClassWakeDegraded, "",
			"⚠ wake deliveries unconfirmed", "reconcile: gtmux digest --json"))
	}
	notify.Send(notify.Options{
		Kind:     "input",
		Title:    i18n.Tr("gtmux HQ is not being woken", "gtmux 中控唤醒失效"),
		Subtitle: "gtmux",
		Message: i18n.Tr("Wake lines aren't reaching the HQ pane — check it for a stuck draft.",
			"唤醒信号没能进入中控窗格 —— 检查输入框是否卡住。"),
	})
}

// resourceTierKey is the dedup key for a machine warning: the tier (amber/red), or
// "" when fine (which clears the marker).
func resourceTierKey(m resource.Machine) string {
	if m.Warn == "" {
		return ""
	}
	return resource.MachineTier(m).String()
}

// resourceHeldPath records the tier the anti-flap rules currently HOLD for the
// machine — the memory a sticky threshold needs (was 14.9 GB a fall out of red, or a
// dither inside its exit band?). Distinct from the gate's own state, which tracks
// belief and speech rather than the reading.
func resourceHeldPath() string { return filepath.Join(state.Dir(), "resourcewarn-held") }

// resourceTierGate runs one machine sample through the three anti-flap rules and
// reports whether to nudge: hysteresis on the thresholds (internal/resource), then
// the confirmation window + restate interval + escalation exemption (the tier gate).
// Single-writer (the serve tick), so its markers have no read-modify-write race.
func resourceTierGate(m resource.Machine, now int64) bool {
	held := tierFromString(state.ReadMarker(resourceHeldPath()))
	sticky := resource.MachineTierSticky(held, m)
	if sticky != held {
		_ = state.WriteMarker(resourceHeldPath(), sticky.String())
	}
	key := ""
	if sticky != resource.TierNormal {
		key = sticky.String()
	}
	return tierGate("resourcewarn", key, now, resource.ConfirmSamples(), resource.MinRestateSecs())
}

// tierFromString parses a persisted tier name (anything unrecognized = normal, which
// is also the correct reading of a first-ever sample).
func tierFromString(s string) resource.Tier {
	switch s {
	case resource.TierAmber.String():
		return resource.TierAmber
	case resource.TierRed.String():
		return resource.TierRed
	default:
		return resource.TierNormal
	}
}

// nudgeHQPane types msg into a live HQ pane, with extra appended when non-empty (the
// stuckDispatchSweep persists a `waiting` marker + fires ONE immediate `waiting` wake
// for a TRACKED dispatch the radar flagged stuck (a startup/permission gate, or its goal
// left unsubmitted in the composer) that has NO hook marker — so the watchdog escalates
// it and the digest shows WHY (kind `startup`/`draft`). Single-writer (slow-tick only);
// the marker-existence check dedups (one wake per stuck episode). It clears when the pane
// un-sticks — `resolveWaiting` removes a stale marker once the pane is genuinely idle.
func stuckDispatchSweep() {
	for _, p := range radar.GatherAgents() {
		// Only a hook-FREE waiting (my radar guard set status but no hook wrote a
		// marker) is a stuck dispatch; a genuine hook wait already owns the marker.
		if p.Status != "waiting" || state.Exists(state.WaitingPath(p.PaneID)) {
			continue
		}
		kind := radar.StuckDispatchKind(p.PaneID, p.Agent)
		if kind == "" {
			continue
		}
		if state.WriteMarker(state.WaitingPath(p.PaneID), kind) == nil {
			nudgeHQPane(hqwake.Line(hqwake.ClassWaiting, p.Loc+" ("+p.PaneID+")",
				"stuck before running — "+kind), "")
		}
	}
}

// resolvedVerdict is resolvedDecide's outcome for one pane sample.
type resolvedVerdict int

const (
	resolvedHold  resolvedVerdict = iota // no change (not tracked, or a flicker to keep tracking)
	resolvedTrack                        // (re)record the pane as waiting on `kind`
	resolvedEmit                         // waiting→clear CONFIRMED: emit resolved + clear tracker/marker
)

// resolvedDecide is the PURE transition rule (no IO, testable). From the pane's
// DISPLAYED status, the kind we last tracked it waiting on, the current waiting-marker
// kind, and whether the screen STILL shows a wait (menu/gate), it decides the
// resolved-backstop action. On resolvedTrack the tracker value is the returned `kind`;
// on resolvedEmit the cleared kind is the caller's lastTracked.
//
// The signal is the DISPLAYED status (resolveWaiting flips a fresh-marked pane to
// "working" once the approved tool runs, even while the hook marker lingers), so a
// permission approved in the source window reads as waiting→working here even though no
// hook cleared the marker. screenWaits is the flicker guard: a one-tick liveWorking flip
// while the approval card is still on screen must NOT read as resolved.
func resolvedDecide(status, lastTracked, markerKind string, screenWaits bool) (resolvedVerdict, string) {
	if status == "waiting" {
		if markerKind == "" {
			markerKind = "pending"
		}
		return resolvedTrack, markerKind
	}
	if lastTracked == "" || screenWaits {
		return resolvedHold, ""
	}
	return resolvedEmit, ""
}

// resolvedTrackPath is the slow-tick's per-pane tracker of the kind it last saw a pane
// waiting on — the state resolvedDecide compares against to detect a waiting→clear edge.
// Single-writer (slow-tick only), so no read-modify-write race.
func resolvedTrackPath(pane string) string {
	return filepath.Join(state.Dir(), "hqwake", "resolved-track-"+pane)
}

// resolvedTransitionSweep fires a `resolved` wake when a pane LEAVES the waiting state
// via a path no hook event covers (hq-send-delivery-reliability B). It is the BACKSTOP
// to the hook's event-gated resolved emit: the hook fires only on
// {UserPromptSubmit,Resumed,Stop,StopFailure}, but a permission/question approved in the
// pane's OWN window and resumed produces none of those promptly (Claude registers no
// PostToolUse), so HQ would hold a stale needs-you until the eventual Stop. This tracks
// each pane's waiting kind and, on a confirmed waiting→non-waiting edge, emits ONE
// resolved — deduped against the hook via hqwake.ClaimResolved and delivered on the
// acked hqnudge channel. On emit it also drops the lingering hook waiting marker (safe:
// the edge is screen-confirmed clear), so a later Stop can't double-announce it.
func resolvedTransitionSweep() {
	now := time.Now().Unix()
	for _, p := range radar.GatherAgents() {
		pane := p.PaneID
		last := state.ReadMarker(resolvedTrackPath(pane))
		markerKind := state.ReadMarker(state.WaitingPath(pane))
		// Capture only for a transition CANDIDATE (was tracked waiting, now not) — the
		// flicker guard, paid once per edge rather than per pane per tick.
		screenWaits := false
		if p.Status != "waiting" && last != "" {
			cap := tmux.CaptureFull(pane)
			screenWaits = prompt.WaitingOptions(cap) != nil || prompt.IsStartupGate(cap, p.Agent)
		}
		switch v, kind := resolvedDecide(p.Status, last, markerKind, screenWaits); v {
		case resolvedTrack:
			if kind != last {
				_ = state.WriteMarker(resolvedTrackPath(pane), kind)
			}
		case resolvedEmit:
			if hqwake.ClaimResolved(pane, now) {
				nudgeHQPane(hqwake.Line(hqwake.ClassResolved, p.Loc+" ("+pane+")", "was "+last), "")
			}
			state.Remove(resolvedTrackPath(pane))
			state.Remove(state.WaitingPath(pane)) // screen-confirmed clear → drop the lingering marker
		}
	}
}

// reclaim hint). For an alert whose dedup already decided it should speak.
func nudgeHQPane(msg, extra string) {
	pane := hqpane.Find()
	if pane == "" {
		return
	}
	if extra != "" {
		msg += " — " + extra
	}
	hqnudge.Deliver(pane, msg) // draft-guarded like every other HQ injection
}

// limitsTierKey reduces a limits warn ("week (fable) 93%") to its window identity
// ("week (fable)") so the dedup fires once per window crossing, not per % it climbs.
// "" (no warn) clears the marker.
func limitsTierKey(warn string) string {
	warn = strings.TrimSpace(warn)
	if warn == "" {
		return ""
	}
	if i := strings.LastIndexByte(warn, ' '); i > 0 {
		return strings.TrimSpace(warn[:i])
	}
	return warn
}

// nudgeOnChange sends msg to a live HQ pane only when the dedupKey DIFFERS from the
// key recorded in the marker; a same-key repeat is suppressed. Pass a TIER/LAYER key
// (not the raw jittering value) so intra-tier drift doesn't re-nudge. extra is
// appended to the message (e.g. the reclaim hint), when non-empty.
func nudgeOnChange(marker, dedupKey, msg, extra string) {
	if !markerChanged(marker, dedupKey) {
		return
	}
	nudgeHQPane(msg, extra)
}

// markerChanged reports whether dedupKey differs from the marker's last value, and
// persists the new key when it does. An empty key CLEARS the marker and returns false
// (nothing to nudge). This is the by-TIER dedup core — testable without tmux.
func markerChanged(marker, dedupKey string) bool {
	path := filepath.Join(state.Dir(), marker)
	if dedupKey == state.ReadMarker(path) {
		return false // unchanged tier → never re-nudge
	}
	if dedupKey == "" {
		state.Remove(path)
		return false
	}
	return state.WriteMarker(path, dedupKey) == nil
}

// orphanTail is the ADVISORY half of a resource warning: one suggestion of what could be
// reclaimed. It is deliberately decoupled from the alarm (standing-wake-backoff §4.2) —
// the alarm is "the machine is short of X" and stands on its own, while the suggestion is
// a guess. The iOS-Simulator-runtime suggestion in particular was measured wrong ×6 while
// being the only information the line carried, which is how a true alarm came to read as
// a false one. So it is marked as a guess, and its absence never withholds the alarm.
func orphanTail(rep resource.Report) string {
	if len(rep.Orphans) == 0 {
		return ""
	}
	o := rep.Orphans[0]
	return "maybe reclaimable: " + o.Comm
}
