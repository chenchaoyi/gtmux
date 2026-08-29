// The HQ SESSION-HEALTH sensor (hq-self-rotate): the one thing that watches the watcher.
//
// Every other sensor in this package asks a question about the fleet. This one asks whether
// the faculty that answers those questions is still working. On 2026-08-03, deep into a long
// session, HQ read its OWN previous turn as the commander's input — it found a line saying
// "that message was from me, don't worry" and withdrew a suspicion it had raised correctly.
// The event stream settles what actually happened, because the two acts have different
// TYPES: seq 7196 was `UserPromptSubmit` on `%4` (the commander), seq 7210 was `Stop` on `%4`
// (HQ's own output). As text they read the same. As events they never could.
//
// Two properties of that failure decide this sensor's whole shape:
//
//   - It cannot be self-detected. The boundary between produced and received is exactly what
//     degrades, so the faculty that would have to notice is the one that failed. The
//     observation has to come from outside the session.
//   - It cannot be self-scheduled. HQ is not running between wakes and owns no timer, so
//     "rotate when you get heavy" is an instruction with nobody to execute it — the failure
//     #647 documented, where correctly-fired sensors ran zero passes for 13 days because the
//     signal reached no resident reader.
//
// So gtmux senses from the resident tick and knocks; HQ, which is the only party that knows
// what it is mid-way through, does the handoff and the rotation itself. The debt clears the
// way the consumption watermark's does — only by the act, never by the knock.
//
// Distinct from `self-check` on purpose: that audits HQ's PRODUCTS (ledger, memory, feed),
// is rate-limited to ≤1/h, and is charter-bound to stay silent. This audits the JUDGE, is
// threshold-driven, and must be heard.
package hq

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// rotateStatePath holds the sensor's memory of the session it is currently watching:
// "<session> <startedAt> <turns> <cursor> <checkedAt> <knockedAt>". Single-writer (the serve
// slow tick), so it needs no locking. A session id has no spaces; "-" stands in for empty.
func rotateStatePath() string { return filepath.Join(state.Dir(), "hqwake", "selfrotate-state") }

// rotateState is the health window for ONE HQ session. Everything in it is scoped to
// Session: when that changes, the whole window is discarded rather than migrated, because a
// rotated session shares nothing with its predecessor except the pane it sits in.
type rotateState struct {
	Session string // the agent session id this window describes
	// StartedAt is when the conversation began — the transcript's own first message when it
	// is readable, else when this sensor first saw the session. Preferring the transcript is
	// what stops a serve restart from resetting a twelve-hour-old session's age to zero,
	// which is precisely when a stale supervisor most needs to be told.
	StartedAt int64
	// Turns counts HQ-pane prompt submissions since the window opened. It accrues from the
	// event stream rather than being recounted, so no tick re-scans the journal from zero.
	// A window adopted mid-session starts at 0 and therefore UNDER-counts: deliberate, since
	// under-counting can only delay a knock, while over-counting would invent one.
	Turns int
	// Cursor is the event sequence through which Turns has already counted.
	Cursor int64
	// CheckedAt paces the only non-trivial work here (the usage snapshot, the log-head read,
	// the event-delta scan); KnockedAt paces the re-knock while a breach stands.
	CheckedAt int64
	KnockedAt int64
	// Breach and KnockMoved are the last DELIVERED knock's fingerprint (see standing.go):
	// which criteria were over the line, and the fleet counter at that moment. A repeat
	// whose fingerprint is unchanged says nothing new and is suppressed.
	Breach     string
	KnockMoved int64
	// Moved is the running count of NON-HQ fleet events this window has seen. Cumulative
	// and compared only against KnockMoved, so its absolute value never matters.
	Moved int64
}

func readRotateState() rotateState {
	f := strings.Fields(state.ReadMarker(rotateStatePath()))
	var st rotateState
	if len(f) >= 6 {
		if f[0] != "-" {
			st.Session = f[0]
		}
		st.StartedAt, _ = strconv.ParseInt(f[1], 10, 64)
		st.Turns, _ = strconv.Atoi(f[2])
		st.Cursor, _ = strconv.ParseInt(f[3], 10, 64)
		st.CheckedAt, _ = strconv.ParseInt(f[4], 10, 64)
		st.KnockedAt, _ = strconv.ParseInt(f[5], 10, 64)
	}
	// Appended by standing-wake-backoff. Read defensively: a marker written by an older
	// binary has six fields, and it must keep parsing rather than reset a live window.
	if len(f) >= 9 {
		if f[6] != "-" {
			st.Breach = f[6]
		}
		st.KnockMoved, _ = strconv.ParseInt(f[7], 10, 64)
		st.Moved, _ = strconv.ParseInt(f[8], 10, 64)
	}
	return st
}

func writeRotateState(st rotateState) {
	sess := st.Session
	if sess == "" {
		sess = "-"
	}
	breach := st.Breach
	if breach == "" {
		breach = "-"
	}
	_ = state.WriteMarker(rotateStatePath(), strings.Join([]string{
		sess,
		strconv.FormatInt(st.StartedAt, 10),
		strconv.Itoa(st.Turns),
		strconv.FormatInt(st.Cursor, 10),
		strconv.FormatInt(st.CheckedAt, 10),
		strconv.FormatInt(st.KnockedAt, 10),
		breach,
		strconv.FormatInt(st.KnockMoved, 10),
		strconv.FormatInt(st.Moved, 10),
	}, " "))
}

// ── the verdict (pure) ───────────────────────────────────────────────────────

// rotateBreach names one crossed threshold, e.g. "ctx 82% ≥ 75%".
type rotateBreach struct {
	What string // "ctx" | "age" | "turns"
	Text string
}

// rotateBreaches reports every threshold the sensed session has crossed. A NON-POSITIVE
// threshold disables that criterion alone — the three measure different kinds of wear
// (context blurs the produced/received boundary, age accumulates stale posture, turns run up
// on a burst-heavy day long before either clock says so), so a user who distrusts one must
// be able to silence it without losing the other two.
func rotateBreaches(ctxFrac float64, ageSec int64, turns int, cfg hqwake.Config) []rotateBreach {
	var out []rotateBreach
	if cfg.SelfRotateCtx > 0 && ctxFrac >= cfg.SelfRotateCtx {
		out = append(out, rotateBreach{"ctx", fmt.Sprintf("ctx %d%% ≥ %d%%",
			pct(ctxFrac), pct(cfg.SelfRotateCtx))})
	}
	if cfg.SelfRotateHours > 0 && ageSec >= cfg.SelfRotateHours*3600 {
		out = append(out, rotateBreach{"age", fmt.Sprintf("age %s ≥ %dh",
			HumanAgeShort(ageSec), cfg.SelfRotateHours)})
	}
	if cfg.SelfRotateTurns > 0 && turns >= cfg.SelfRotateTurns {
		out = append(out, rotateBreach{"turns", fmt.Sprintf("turns %d ≥ %d",
			turns, cfg.SelfRotateTurns)})
	}
	return out
}

// pct renders a 0–1 fraction as whole percent.
func pct(f float64) int { return int(f*100 + 0.5) }

// rotateVerdict is selfRotateDecide's outcome for one evaluation.
type rotateVerdict int

const (
	rotateHealthy rotateVerdict = iota // nothing crossed — stay silent
	rotateHold                         // breached, but inside the re-knock interval
	rotateKnock                        // breached and due — deliver
)

// selfRotateDecide is the PURE cadence decision (no clock, no disk, no tmux). The state it
// returns is what to persist. Note what it does NOT do: nothing here clears the breach.
// Only a new session id does, in the caller — which is the same guarantee the consumption
// watermark makes, applied to a different debt. gtmux stops asking when the act it asked for
// has happened, not when it has finished asking.
func selfRotateDecide(now int64, breaches []rotateBreach, st rotateState, repeat, floor int64) (rotateVerdict, rotateState) {
	w := standingWorld{Breach: breachSet(breaches), Moved: st.Moved}
	v, next := standingDecide(now, w, standingState{
		KnockedAt: st.KnockedAt, Breach: st.Breach, Moved: st.KnockMoved}, repeat, floor)
	st.KnockedAt, st.Breach, st.KnockMoved = next.KnockedAt, next.Breach, next.Moved
	switch v {
	case standingKnock:
		return rotateKnock, st
	case standingHold:
		return rotateHold, st
	default:
		return rotateHealthy, st
	}
}

// breachSet renders the crossed criteria as the stable identity the standing rule compares:
// the WHAT keys in the fixed order rotateBreaches emits them, never the figures. `age 13h ≥
// 12h` re-renders differently every hour while describing the same unchanged fact, and a
// fingerprint built from it would re-arm the alarm forever — which is the loop this change
// exists to close.
func breachSet(bs []rotateBreach) string {
	parts := make([]string, 0, len(bs))
	for _, b := range bs {
		parts = append(parts, b.What)
	}
	return strings.Join(parts, ",")
}

// ── sensing ──────────────────────────────────────────────────────────────────

// hqSessionRef resolves the HQ pane's (agent, session id) through the same record the radar
// and digest use — the resume record the hook wrote, keyed by the pane's location. Empty
// when the pane has not run a turn yet (a just-launched HQ), which is not an error: there is
// simply no session to judge.
func hqSessionRef(pane string) (agent, sessionID string) {
	loc := strings.TrimSpace(tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}"))
	if loc == "" {
		return "", ""
	}
	if rec, ok := resume.Load(loc); ok {
		return rec.Agent, rec.SessionID
	}
	return "", ""
}

// countHQTurns counts the HQ pane's prompt submissions past `cursor` and returns them with
// the highest sequence scanned. Every delivered wake lands back in the stream as one of
// these, and that is CORRECT here (unlike the unread sensor, which must exclude them): a
// wake HQ answered consumed a turn's worth of context exactly like any other.
// countHQTurns walks the event delta once and reports both things this sensor needs from
// it: HQ's own prompt submissions (turns, a wear criterion) and how many records came from
// ANYWHERE ELSE (fleet movement, the standing-knock world signal). One pass, because the
// second question arrived later and re-scanning for it would double the sensor's only
// non-trivial read.
func countHQTurns(pane string, cursor int64) (n int, fleet int64, maxSeq int64) {
	recs, _ := events.ReadSince(cursor)
	maxSeq = cursor
	for _, r := range recs {
		if r.Seq > maxSeq {
			maxSeq = r.Seq
		}
		if r.Pane == pane {
			if r.Event == "UserPromptSubmit" {
				n++
			}
			continue // HQ's own records are never fleet movement — see standingWorld.Moved
		}
		if events.IsControl(r) {
			// gtmux's own bookkeeping — maintenance triggers and the audit trail — is
			// never fleet movement (the events.go sensor doctrine). Without this, every
			// delivered wake's own audit record would advance Moved and re-arm the very
			// standing knock that produced it: the alarm manufacturing its own evidence.
			continue
		}
		fleet++
	}
	return n, fleet, maxSeq
}

// ── the slow-tick sensor ─────────────────────────────────────────────────────

// selfRotateSensor is the slow-tick entry point. Cheap by construction: the pacing gate is
// one small file read, and everything expensive sits behind it.
func selfRotateSensor(now int64) {
	pane := hqpane.Find()
	if pane == "" {
		// No supervisor — nothing to watch and nothing to say. Critically, the window is NOT
		// touched either: an absent HQ must never be recorded as a rotated one, or the
		// session that comes back would be judged as brand new.
		return
	}
	agent, sess := hqSessionRef(pane)
	selfRotateSensorFor(pane, sess, ctxFracFor(agent, sess),
		transcript.FirstMessageTime(agent, sess), now)
}

// ctxFracFor is the live context fraction for a session (0 when there is no usage data —
// a non-Claude agent, or a log that has not been written yet).
func ctxFracFor(agent, sessionID string) float64 {
	if sessionID == "" {
		return 0
	}
	if u, ok := uwatch.ForSession(agent, sessionID, time.Now()); ok {
		return u.CtxFrac
	}
	return 0
}

// selfRotateSensorFor is the sensor body over already-sensed facts — the seam that lets the
// whole path, delivery queue and all, be tested without a tmux server or an agent log.
func selfRotateSensorFor(pane, sessionID string, ctxFrac float64, firstMsgAt, now int64) {
	cfg := hqwake.Load()
	st := readRotateState()
	if st.CheckedAt != 0 && now-st.CheckedAt < cfg.SelfRotateCheckSec {
		return // pacing gate: the 20 s tick is far finer than anything measured here
	}
	if sessionID == "" {
		// The hook has recorded no session for this pane yet. Nothing to judge — and nothing
		// to reset, since we do not know what we are looking at.
		st.CheckedAt = now
		writeRotateState(st)
		return
	}
	if sessionID != st.Session {
		// A new session — either the first one we have seen, or the rotation we asked for.
		// Either way the previous window describes a conversation that no longer exists, so
		// it is discarded whole: age restarts, turns restart, and the knock cadence resets.
		// The journal keeps the chain the discard would otherwise destroy: the predecessor
		// id is about to be overwritten everywhere else (this state file, the resume
		// record), and it is the only pointer to the retiring session's transcript. A
		// FIRST sighting is not a replacement — nothing is being lost, and a healthy
		// first sight writes nothing to the stream (the sensors' silence discipline).
		if st.Session != "" {
			events.AuditHQSession(sessionID, st.Session, now)
		}
		st = rotateState{Session: sessionID, StartedAt: rotateStart(firstMsgAt, now),
			Cursor: events.CurrentSeq()}
	}
	n, fleet, maxSeq := countHQTurns(pane, st.Cursor)
	st.Turns, st.Cursor, st.CheckedAt = st.Turns+n, maxSeq, now
	st.Moved += fleet

	ageSec := int64(0)
	if st.StartedAt > 0 {
		ageSec = now - st.StartedAt
	}
	breaches := rotateBreaches(ctxFrac, ageSec, st.Turns, cfg)
	v, next := selfRotateDecide(now, breaches, st, cfg.SelfRotateRepeatSec, cfg.SelfRotateFloorSec)
	writeRotateState(next)
	if v != rotateKnock {
		return
	}
	hqnudge.Deliver(pane, hqwake.Line(hqwake.ClassSelfRotate,
		rotateFigures(ctxFrac, ageSec, next.Turns),
		i18n.Tr("over: ", "越线: ")+rotateBreachText(breaches),
		i18n.Tr("board+KB current → hand off → gtmux hq --rotate",
			"先把看板与知识库写到最新 → 交接 → gtmux hq --rotate")))
}

// rotateStart resolves when the session began: the transcript's own first message when it
// is readable, else now. The fallback under-states an adopted session's age, which can only
// ever delay a knock — the safe direction for a signal the user has to believe.
func rotateStart(firstMsgAt, now int64) int64 {
	if firstMsgAt > 0 {
		return firstMsgAt
	}
	return now
}

// rotateFigures renders the sensed snapshot as the wake line's head: "ctx 82% · 14h · 380
// turns". The FIGURES lead rather than the verdict, because HQ is being asked to act on its
// own state and has to be able to check the claim.
//
// A NEGATIVE turns means "not counted yet" and is omitted rather than printed as 0: the
// count only accrues once the sensor has opened a window for this session, and a confident
// "0 turns" beside a two-day-old session is the kind of small lie that costs a row its
// credibility.
//
// A ZERO ctx is omitted for the same reason, and it matters more. `ctxFracFor` returns 0
// when there is no usage data — which is EVERY non-Claude agent, because the usage parser
// reads Claude's log shape. So an HQ running on codex or opencode was told
// `ctx 0% · 14h · 380 turns`, and "0%" reads as "plenty of room": evidence AGAINST
// rotating, inside the one mechanism built to catch a judge that cannot self-assess. The
// criterion itself never misfired (0 is below any threshold); what it did was undercut a
// correct rotation. A live session never genuinely sits at 0, so omission is unambiguous.
func rotateFigures(ctxFrac float64, ageSec int64, turns int) string {
	var parts []string
	if ctxFrac > 0 {
		parts = append(parts, fmt.Sprintf("ctx %d%%", pct(ctxFrac)))
	}
	if ageSec > 0 {
		parts = append(parts, HumanAgeShort(ageSec))
	}
	if turns >= 0 {
		parts = append(parts, strconv.Itoa(turns)+i18n.Tr(" turns", " 轮"))
	}
	return strings.Join(parts, " · ")
}

// rotateBreachText joins the crossed thresholds for the wake line.
func rotateBreachText(bs []rotateBreach) string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.Text)
	}
	return strings.Join(out, ", ")
}

// ── rotation (the act HQ performs) ───────────────────────────────────────────

// RotateHQ delivers the agent's own conversation-reset input into the live HQ pane and
// restarts the health window. It is the mechanism behind `gtmux hq --rotate`, and it exists
// so that self-rotation does not require weakening HQ's role boundary: HQ decides and hands
// off, gtmux performs the tmux mechanics, and the rule that HQ runs no concrete command and
// never sends keys into a TUI stands untouched.
//
// It refuses to type into a box that is not confirmed empty — see rotateHQ.
//
// Delivery is paste-then-Enter as separate steps (the swallowed-Enter lesson), and it is
// deliberately UNVERIFIED: the turn being reset is the one calling this, so there is no
// later hook event to confirm against from in here. Success is therefore not assumed — it
// only holds the knock for one repeat interval, long enough for the new session id to
// appear. If the reset did not take, the id does not change and the sensor knocks again,
// which is the correct recovery.
func RotateHQ() (input string, ok bool, held string) {
	pane := hqpane.Find()
	if pane == "" {
		return "", false, ""
	}
	return rotateHQ(pane, dispatchbridge.DispatchIO(pane))
}

// rotateHQ is RotateHQ over an injectable pane I/O — the same seam every other delivery
// uses, so the guard here is the guard there.
//
// The box is checked BEFORE anything is typed. A rotation types `/clear` and submits it,
// so on a box that already holds the user's half-written line it does not reset a
// session — it submits that line, joined to a slash command, as an instruction. That is
// what happened on 2026-08-29: a half-typed `%11 ` became `%11 /clear`, HQ read it as an
// order and cleared a real session. Withholding costs nothing by comparison: the session
// id does not change, the sensor knocks again, and the rotation happens at the next
// attempt with an empty box.
func rotateHQ(pane string, io dispatch.IO) (string, bool, string) {
	if !dispatch.BoxEmpty(io) {
		return "", false, i18n.Tr(
			"that pane has unsent text in its input box — rotating would submit it",
			"该 pane 的输入框里有未提交的内容 —— 现在轮换会把它一起交出去")
	}
	agent, retiring := hqSessionRef(pane)
	input := rotateInput(agent)
	if io.ExitMode != nil {
		_ = io.ExitMode() // keys are swallowed while the pane is in copy/view-mode
	}
	if io.Paste(input) != nil {
		return "", false, ""
	}
	if io.Enter() != nil {
		return "", false, ""
	}
	// The act enters the journal with the retiring id — the settle record (the sensor
	// observing the successor) completes the chain; this one also catches a rotation
	// that never settles, which is itself worth being able to see.
	events.AuditRotate(retiring, input, time.Now().Unix())
	st := readRotateState()
	st.KnockedAt = time.Now().Unix()
	writeRotateState(st)
	return input, true, ""
}

// rotateInput is the agent's own "start a fresh conversation" command. Unknown agents get
// Claude's `/clear`, which is harmless where it is not recognized (the agent reports an
// unknown command and the session simply does not rotate — the sensor then knocks again).
func rotateInput(agent string) string {
	switch agent {
	case "codex":
		return "/new"
	default:
		return "/clear"
	}
}

// ── the read side `gtmux doctor` reports ─────────────────────────────────────

// SessionHealthRow is the supervisor's own-session verdict (consumed by `gtmux doctor`).
type SessionHealthRow struct {
	CtxFrac float64
	AgeSec  int64
	// Turns is -1 when the sensor has not yet opened a health window for this session, so
	// the count is genuinely unknown rather than zero.
	Turns int
	Over  string           // the crossed thresholds, "" when healthy
	State MaintenanceState // Never (no HQ / no session yet) | OK | Slipped
}

// SessionHealthStatus reports whether the supervisor's own session is still fit to judge —
// the observability half of the guarantee, and the reason the user does not have to be the
// detector. Read-only: the sensor is the single writer of the health window, so a doctor run
// can never move the cadence it is reporting on.
//
// With no live HQ, or no session resolvable for it, the verdict is NEVER rather than a
// failure. An absent supervisor is not a degraded one, and a doctor that cried wolf on every
// machine with an HQ home and no HQ running would be ignored on the day it was right.
func SessionHealthStatus(now int64) SessionHealthRow {
	pane := hqpane.Find()
	if pane == "" {
		return SessionHealthRow{State: MaintenanceNever}
	}
	agent, sess := hqSessionRef(pane)
	if sess == "" {
		return SessionHealthRow{State: MaintenanceNever}
	}
	st := readRotateState()
	turns, startedAt := -1, transcript.FirstMessageTime(agent, sess)
	if st.Session == sess {
		turns = st.Turns
		if st.StartedAt > 0 {
			startedAt = st.StartedAt
		}
	}
	age := int64(0)
	if startedAt > 0 {
		age = now - startedAt
	}
	return sessionHealthRow(ctxFracFor(agent, sess), age, turns, hqwake.Load())
}

// sessionHealthRow is the PURE verdict over sensed figures (no clock, no disk — the testable
// core), and it is the SAME rotateBreaches call the sensor makes, so the row and the knock
// can never disagree about whether this session is worn out.
func sessionHealthRow(ctxFrac float64, ageSec int64, turns int, cfg hqwake.Config) SessionHealthRow {
	row := SessionHealthRow{CtxFrac: ctxFrac, AgeSec: ageSec, Turns: turns, State: MaintenanceOK}
	if bs := rotateBreaches(ctxFrac, ageSec, turns, cfg); len(bs) > 0 {
		row.Over, row.State = rotateBreachText(bs), MaintenanceSlipped
	}
	return row
}

// Figures renders the row the way the wake line does, so doctor and the knock never disagree
// about what they are describing.
func (r SessionHealthRow) Figures() string { return rotateFigures(r.CtxFrac, r.AgeSec, r.Turns) }
