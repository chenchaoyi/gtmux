// Usage-watch evaluation at hook time (usage-watch change). Hooks fire on every
// lifecycle event — during real work that's every tool call, near-real-time for
// burn — so each event cheaply refreshes the session's usage counters (byte-
// incremental) and evaluates the SESSION layers (ctx / burn, with projection).
//
// Outcome is a state marker (usagewarn/<pane>, content = the warn string) — the
// radar reads it as an amber modifier like errored/bg — plus ONE nudge line into
// a live hq pane when the warn FIRST appears or names a different layer (the
// marker doubles as the dedup, mirroring the waiting nudge's priorKind check).
// The known P1 gap (a long silent generation fires no hooks until Stop) is
// recorded in the change; the serve tick is the P2 refinement.
package hook

import (
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/usage"
)

// watchUsage refreshes + evaluates usage for the session behind this hook event
// and maintains the pane's warn marker. agentKey/sessionID from the payload;
// pane may be "" (native session — sensed, but no pane marker to hang the amber
// modifier on; skipped in P1).
func watchUsage(agentKey, sessionID, pane string) {
	if pane == "" || sessionID == "" {
		return
	}
	s, ok := usage.ForSession(agentKey, sessionID, time.Now())
	if !ok {
		return
	}
	warn := usage.EvaluateSession(s)
	prior := state.ReadMarker(state.UsageWarnPath(pane))
	switch {
	case warn == "" && prior != "":
		state.Remove(state.UsageWarnPath(pane)) // back under every layer
	case warn != "" && layerOf(warn) != layerOf(prior):
		// First breach, or a DIFFERENT layer than last time → mark + nudge once.
		_ = state.WriteMarker(state.UsageWarnPath(pane), warn)
		nudgeUsage(pane, warn)
	case warn != "":
		// Same layer, refreshed detail (e.g. ctx 82% → 84%) → update quietly.
		_ = state.WriteMarker(state.UsageWarnPath(pane), warn)
	}
}

// layerOf collapses a warn string to its layer identity for dedup: "ctx 86%"
// and "ctx→80% in ~9m" are both the ctx layer.
func layerOf(warn string) string {
	switch {
	case warn == "":
		return ""
	case strings.HasPrefix(warn, "ctx"):
		return "ctx"
	case strings.HasPrefix(warn, "burn"):
		return "burn"
	default:
		return warn
	}
}

// nudgeUsage wakes a live hq pane about a session crossing a usage layer.
//
// It used to hand-build `[gtmux] usage·warn …` and `tmux.SendText(target, msg, true)`
// it straight into the pane — bypassing the wake channel entirely. That meant no draft
// guard: a warning firing while the user was mid-sentence in HQ appended itself to their
// draft AND pressed Enter, which is the exact data loss hqnudge exists to prevent. It
// also skipped the ack/retry, the copy-mode check, coalescing and priority, used a format
// retired by hq-perception-v2, and named a class no playbook has ever taught. It survived
// two channel rewrites because the one thing it did NOT do was go through them.
//
// The warn string is gtmux's own computed summary ("ctx 86%"), not agent- or
// user-authored, so it needs no DATA quoting — same as resource·warn / limits·warn.
func nudgeUsage(pane, warn string) {
	nudgeHQ(pane, hqwake.Line(hqwake.ClassUsageWarn, wakeHead(pane), warn))
}
