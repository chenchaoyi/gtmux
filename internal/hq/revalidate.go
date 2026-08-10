package hq

// Delivery-side premise probes (standing-wake-backoff §2.1/§4.1/§5.2).
//
// A wake is decided when it is ENQUEUED and delivered when the channel next gets a
// chance to type — behind a half-written draft, behind an earlier batch, behind the
// fast tick. The world moves in that gap, and the line then arrives asserting a state
// that no longer exists. The measured case: `usage·warn … ctx→80% in ~4m` reached HQ
// while the pane's own status bar read 34%, because the harness had auto-compacted
// mid-flight. HQ caught that one by re-reading the live value before printing — a
// consumer-side rule that works only for a consumer who remembers to apply it.
//
// So the check moves to the producer, at the last possible moment before the paste.
// These probes are registered into internal/hqnudge (injection, so the layering stays
// acyclic: hq → hqnudge, never the reverse).
//
// Design rule for every probe here: only DROP on positive evidence that the premise is
// gone. An unreadable sample, an unresolvable pane, anything ambiguous — deliver. The
// failure this change exists to prevent is noise; the failure it must not introduce is
// a real alarm silently discarded on a probe's bad day.

import (
	"regexp"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resource"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// RegisterWakeProbes wires the delivery-side premise probes. Called once from serve's
// startup, before any tick runs.
func RegisterWakeProbes() {
	hqnudge.RegisterRevalidator(resourceWarnProbe)
	hqnudge.RegisterRevalidator(usageWarnProbe)
}

// resourceWarnProbe drops a queued `resource·warn` whose tier has since RECOVERED.
//
// The alarm's whole content is "the machine is at <tier>". If, by the time we can type,
// the machine is no longer at that tier, the sentence is false — and a false alarm on a
// channel whose credibility is the point costs more than the missed notification. A tier
// that has WORSENED still delivers: the line understates it, which is the harmless
// direction, and the next sample raises the escalation that never gets suppressed.
func resourceWarnProbe(line string) (bool, string) {
	if !strings.Contains(line, hqwake.ClassResourceWarn) {
		return true, line // not ours
	}
	m := radar.CurrentResource().Machine
	if m.Warn == "" && resource.MachineTier(m) == resource.TierNormal {
		return false, "" // positive evidence: the machine is fine now
	}
	return true, line
}

// wakePaneRe pulls the pane id out of a wake line's head ("sat:0.0 (%74) │ …").
var wakePaneRe = regexp.MustCompile(`\((%\d+)\)`)

// usageWarnProbe re-samples a queued `usage·warn` against the live session.
//
// This is the case that named the whole problem: `ctx→80% in ~4m` reached HQ while the
// pane's own status bar read 34%, because the harness auto-compacted while the warning
// sat in the queue. ctx has an external reset event that neither gtmux nor HQ controls,
// so no amount of care at DECISION time can make the claim true at DELIVERY time. The
// only fix that holds is to look again just before speaking.
//
// Three outcomes, and the middle one is why §2.1 is a re-render seam and not just a drop
// filter: gone → drop; changed → deliver the CURRENT figure rather than the stale one;
// same → deliver untouched.
//
// The dangerous failure here is resolving the WRONG session and discarding a live alarm,
// so every step that cannot be established with certainty delivers: no pane id in the
// line, no session recorded for that pane, no usage snapshot — all pass through.
func usageWarnProbe(line string) (bool, string) {
	if !strings.Contains(line, hqwake.ClassUsageWarn) {
		return true, line // not ours
	}
	head, stale, ok := splitWakeTail(line)
	if !ok {
		return true, line // an unfamiliar shape is not something to judge
	}
	m := wakePaneRe.FindStringSubmatch(head)
	if m == nil {
		return true, line
	}
	agent, sessionID := hqSessionRef(m[1])
	if sessionID == "" {
		return true, line // the pane has no session on record — cannot re-sample
	}
	s, okSess := uwatch.ForSession(agent, sessionID, time.Now())
	if !okSess {
		return true, line // no snapshot — say nothing about it
	}
	switch fresh := uwatch.EvaluateSession(s); {
	case fresh == "":
		return false, "" // positive evidence: nothing is over any line now
	case fresh != stale:
		return true, head + hqwake.FieldSep() + fresh // speak the current truth
	default:
		return true, line
	}
}

// splitWakeTail splits a rendered wake line into everything before its LAST field and
// that field. The usage classes carry exactly one field (the warn), so the last one is
// it; a line with no field at all is not one of ours to rewrite.
func splitWakeTail(line string) (head, tail string, ok bool) {
	i := strings.LastIndex(line, hqwake.FieldSep())
	if i < 0 {
		return "", "", false
	}
	return line[:i], line[i+len(hqwake.FieldSep()):], true
}

// probeForTest runs the registered probe chain the way hqnudge does — first probe that
// changes anything wins — so tests exercise the real composition, not one probe.
func probeForTest(line string) (bool, string) {
	for _, f := range []func(string) (bool, string){resourceWarnProbe, usageWarnProbe} {
		if keep, out := f(line); !keep || out != line {
			return keep, out
		}
	}
	return true, line
}
