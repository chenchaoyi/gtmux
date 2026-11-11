package radar

import (
	"os"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/ansi"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// codexStaleActiveDisplay is deliberately weaker than lifecycle reconciliation:
// it corrects a row, never authorizes a Stop, a notification or marker cleanup.
// A newer finished cache plus a ready, nonworking pane contradict an obsolete
// active marker even when its missing/corrupt frontend prevents rollout proof.
// Verified frontends continue through the existing exact-session/turn rules.
func codexStaleActiveDisplay(pane, frame string, now int64, cpuWorking bool) (int64, bool) {
	if cpuWorking || state.Exists(state.WaitingPath(pane)) ||
		!prompt.IsComposerReady(frame, "codex") || prompt.WaitingOptions(frame) != nil || codexScreenWorking(frame) {
		return 0, false
	}
	if _, live := foreignCodexFrontend(pane); live {
		return 0, false
	}
	active, err := os.Stat(state.ActivePath(pane))
	if err != nil || !active.ModTime().Before(time.Unix(now-2, 0)) {
		return 0, false // a newly accepted turn can precede its first Working frame
	}
	sid, turn := state.ReadMarker(state.ActivePath(pane)), state.ReadMarker(state.TurnIDPath(pane))
	finished, err := os.Stat(state.FinishedPath(pane))
	if sid == "" || err != nil || !finished.ModTime().After(active.ModTime()) || finished.ModTime().After(time.Unix(now, 0)) {
		return 0, false
	}
	// Do not override a start/wait that changed while gathering this observation.
	check, err := os.Stat(state.ActivePath(pane))
	if err != nil || !check.ModTime().Equal(active.ModTime()) || state.ReadMarker(state.ActivePath(pane)) != sid ||
		state.ReadMarker(state.TurnIDPath(pane)) != turn || state.Exists(state.WaitingPath(pane)) {
		return 0, false
	}
	return finished.ModTime().Unix(), true
}

// Only the current footer's elapsed value can date current visible work when a
// start marker predates a finished observation. Transcript prose is not a clock.
func codexWorkingSince(frame string, now int64) (int64, bool) {
	lines := strings.Split(ansi.Strip(frame), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "Worked for ") {
			return 0, false
		}
		const prefix = "• Working ("
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, " • esc to interrupt)") {
			continue
		}
		elapsed := strings.TrimSuffix(strings.TrimPrefix(line, prefix), " • esc to interrupt)")
		d, err := time.ParseDuration(strings.ReplaceAll(elapsed, " ", ""))
		if err != nil || d < 0 || d > 365*24*time.Hour {
			return 0, false
		}
		return now - int64(d/time.Second), true
	}
	return 0, false
}
