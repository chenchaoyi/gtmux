package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
)

// A phone send never waives the draft guard. Its sendID made it pass force = true to
// DeliverOpts, which maps the operator's --force onto BOTH refusals, so since #734 every
// phone send carried ClobberDraft and pasted onto whatever the Mac's user was typing.
// The phone keeps the interlock waiver (its sendID is its idempotency) and nothing else.
func TestPhoneSendKeepsTheDraftGuard(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // LoadTuning reads the config
	tune := dispatch.LoadTuning()
	for _, sendID := range []string{"phone-1", ""} {
		o := phoneDeliverOpts("%5", "claude", sendID, tune)
		if o.ClobberDraft {
			t.Errorf("sendID %q: ClobberDraft is set; a phone send must hit the draft guard", sendID)
		}
		if o.Force != (sendID != "") {
			t.Errorf("sendID %q: Force = %v", sendID, o.Force)
		}
		if !o.HasComposer {
			t.Errorf("sendID %q: a known agent's pane has a composer to guard", sendID)
		}
	}
	// The operator's own --force still waives both, as documented.
	if o := dispatchbridge.DeliverOpts("%5", "claude", true, tune); !o.ClobberDraft || !o.Force {
		t.Errorf("gtmux send --force: %+v", o)
	}
}
