package app

import "testing"

// Two continuum save triggers in status-right means every interval runs the save twice,
// forever, with nothing on screen to say so. It has a specific cause worth pinning:
// continuum decides whether to inject by looking for its OWN ABSOLUTE path, so a trigger
// written by hand as `~/…` doesn't match and it appends a second, absolute copy.
//
// gtmux never writes status-right — it only reads it — so it cannot prevent the second
// injection. Noticing is the job, and noticing requires counting, not just presence.

const (
	tildeTrigger = "#(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)"
	absTrigger   = "#(/Users/x/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)"
)

// The reported scenario, in its own terms.
func TestTildeAndAbsoluteAreTwoTriggersNotOne(t *testing.T) {
	sr := "#{prefix_highlight} " + tildeTrigger + " " + absTrigger
	if n := continuumTriggerCount(sr); n != 2 {
		t.Errorf("count = %d; want 2 — a `~` trigger and an absolute one are two separate triggers, and the save runs twice", n)
	}
	// Presence must still read as armed: autosave IS on, it's just doubled.
	if !statusRightHasContinuumTrigger(sr) {
		t.Error("a doubled trigger must still count as armed — autosave is on, not off")
	}
}

// Either spelling ALONE is a correctly armed setup. The count must not depend on which
// path form was used, or doctor would tell a healthy `~`-only setup that autosave is off.
func TestEitherPathFormAloneIsExactlyOneTrigger(t *testing.T) {
	for name, sr := range map[string]string{
		"tilde":    "#{?client_prefix,^, } " + tildeTrigger,
		"absolute": "#{?client_prefix,^, } " + absTrigger,
		// A relative/other spelling of the same script still counts once.
		"relative": "#(.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)",
	} {
		t.Run(name, func(t *testing.T) {
			if n := continuumTriggerCount(sr); n != 1 {
				t.Errorf("count = %d; want 1", n)
			}
			if !statusRightHasContinuumTrigger(sr) {
				t.Error("want armed")
			}
		})
	}
}

func TestNoTriggerIsZero(t *testing.T) {
	for _, sr := range []string{"", "#{prefix_highlight} %H:%M", "#(gtmux status)"} {
		if n := continuumTriggerCount(sr); n != 0 {
			t.Errorf("continuumTriggerCount(%q) = %d; want 0", sr, n)
		}
		if statusRightHasContinuumTrigger(sr) {
			t.Errorf("status-right %q must not read as armed", sr)
		}
	}
}

// Three is possible (a second reload appends again) and must be reported as such, not
// clamped to "some".
func TestMoreThanTwoIsCountedExactly(t *testing.T) {
	sr := tildeTrigger + " " + absTrigger + " " + absTrigger
	if n := continuumTriggerCount(sr); n != 3 {
		t.Errorf("count = %d; want 3", n)
	}
}
