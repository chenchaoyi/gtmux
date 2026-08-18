// Live-pane hygiene: gtmux's pane-keyed state is reconciled against the panes tmux
// ACTUALLY has, not merely aged out.
//
// The distinction matters because the failure this fixes is not disk growth, it is
// IDENTITY. A tmux pane id is a per-server sequence number: restart the server and %25
// is handed to some other project's pane, while gtmux's records still name %25 and now
// describe the wrong session. Age can't see that — a two-week-old record and a
// ten-minute-old one are equally wrong the moment the number is reissued — so the sweep
// keys on liveness (see state.ReapDeadPaneState) and the churn-marker age-out in
// diskhygiene.go stays as the belt for a machine where serve rarely runs.
package hq

import (
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// deadPaneSweepInterval throttles the sweep. Reconciliation only has work to do when
// panes CLOSE, which is rare next to the 20 s tick — and the moment it matters most (a
// server restart reissuing every number) is covered immediately by restore's own sweep,
// so this is the standing backstop for every other way a pane goes away.
const deadPaneSweepInterval int64 = 5 * 60

// deadPaneSweepLastPath records the last sweep (unix seconds).
func deadPaneSweepLastPath() string { return state.Dir() + "/dead-pane-sweep-last" }

// deadPaneSweep drops pane-keyed state whose pane tmux no longer has. Time-gated, silent
// housekeeping — it raises no wake: a reaped record is the absence of a problem, not one.
func deadPaneSweep(now int64) int {
	if now-state.ReadInt64Marker(deadPaneSweepLastPath()) < deadPaneSweepInterval {
		return 0
	}
	_ = state.WriteInt64Marker(deadPaneSweepLastPath(), now)
	return state.ReapDeadPaneState(tmux.LivePaneIDs())
}

// ReapDeadPaneStateNow runs the sweep unconditionally against the live server — the
// entry point for the one moment that cannot wait for the tick: a restore has just
// rebuilt the fleet on a fresh server, so every id in the old records has either been
// retired or handed to a different pane. Returns how many files were removed.
func ReapDeadPaneStateNow() int { return state.ReapDeadPaneState(tmux.LivePaneIDs()) }
