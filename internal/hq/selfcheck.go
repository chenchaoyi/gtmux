// The HQ self-check sensor (hq-attention-system §8): gtmux SENSES, LLM-free in the
// serve slow-tick, when HQ should run a self-maintenance pass and raises a
// `self-check` control record into the feed. HQ (the LLM) does the actual review and
// cleanup on its OWN artifacts and briefs only on real action — gtmux never runs the
// cleanup itself (no LLM in the timing loop; the split of §5). Deliberately
// infrequent: rate-limited to ≤ 1/h, and the expensive condition reads happen only
// after the cheap rate-limit gate passes.
package hq

import (
	"path/filepath"
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqfeed"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Self-check timing (seconds) — deliberately coarse (design §8.2).
const (
	selfCheckMinInterval = 60 * 60      // rate limit: at most one self-check per hour
	selfCheckIdleQuiet   = 2 * 60 * 60  // "no attention needed" window (resting user)
	selfCheckIdleSince   = 12 * 60 * 60 // and this long since the last self-check
	selfCheckDailyFloor  = 24 * 60 * 60 // a daily pass no matter what
	selfCheckLedgerCap   = 200          // open-ledger threshold that triggers immediately
)

func selfCheckAtPath() string { return filepath.Join(state.Dir(), "hq-feed", "last-self-check") }

func readSelfCheckAt() int64 {
	n, _ := strconv.ParseInt(state.ReadMarker(selfCheckAtPath()), 10, 64)
	return n
}

func writeSelfCheckAt(now int64) {
	_ = state.WriteInt64Marker(selfCheckAtPath(), now)
}

// shouldSelfCheck is the pure decision (design §8.2), testable without tmux/disk. It
// assumes the caller wants a decision now; the rate limit (≤ 1/h) is enforced here so
// the test covers it. Precedence once past the rate limit: a THRESHOLD trip fires
// immediately; else a DAILY floor; else the IDLE (resting-user) trigger.
func shouldSelfCheck(now, lastCheck int64, recentAttention bool, openLedger int, journalOver, cursorGap bool) (bool, string) {
	if now-lastCheck < selfCheckMinInterval {
		return false, "" // rate limited
	}
	if openLedger > selfCheckLedgerCap || journalOver || cursorGap {
		return true, "threshold"
	}
	if now-lastCheck >= selfCheckDailyFloor {
		return true, "daily"
	}
	// Idle: nothing needed the user's attention for a while AND it's been long enough.
	if !recentAttention && now-lastCheck >= selfCheckIdleSince {
		return true, "idle"
	}
	return false, ""
}

// recentAttentionEvent reports whether any notable-or-important event landed in the
// last idle-quiet window — the LLM-free proxy for "the user was recently pinged".
func recentAttentionEvent(now int64) bool {
	for _, r := range events.Read(selfCheckIdleQuiet, now) {
		if events.SeverityRank(r.Severity) >= events.SeverityRank(events.SevNotable) {
			return true
		}
	}
	return false
}

// selfCheckSensor raises a self-check trigger to HQ when due. It runs from the serve
// slow-tick; only with a live HQ (nothing to trigger otherwise), and the expensive
// condition reads run only after the cheap rate-limit gate passes.
func selfCheckSensor(now int64) {
	if hqpane.Find() == "" {
		return
	}
	lastCheck := readSelfCheckAt()
	if now-lastCheck < selfCheckMinInterval {
		return // cheap rate-limit gate — skip the condition reads entirely
	}
	recentAttn := recentAttentionEvent(now)
	openLedger := len(dispatch.ListTasks())
	journalOver := events.OverCeiling()
	_, gap := events.ReadSince(hqfeed.ReadCursor())
	fire, reason := shouldSelfCheck(now, lastCheck, recentAttn, openLedger, journalOver, gap)
	if !fire {
		return
	}
	writeSelfCheckAt(now)
	sev := events.SevNotable
	if journalOver || gap {
		sev = events.SevImportant // a broken log / cursor gap is a severe finding — surface it
	}
	hqfeed.EmitControl(hqfeed.ControlSelfCheck,
		"self-check due ("+reason+") — review feed/ledger/memory health, clean silently, brief only on real action",
		sev, now)
}
