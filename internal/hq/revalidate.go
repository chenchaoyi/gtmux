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
	"strings"

	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resource"
)

// RegisterWakeProbes wires the delivery-side premise probes. Called once from serve's
// startup, before any tick runs.
func RegisterWakeProbes() {
	hqnudge.RegisterRevalidator(resourceWarnProbe)
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

// probeForTest exposes the probe chain to tests in this package without exporting it.
func probeForTest(line string) (bool, string) { return resourceWarnProbe(line) }
