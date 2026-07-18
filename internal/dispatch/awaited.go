package dispatch

import "github.com/chenchaoyi/gtmux/internal/state"

// The AWAITED registry (done-wake-keyed-on-awaited): panes HQ has dispatched work to
// and is expecting a completion from. It exists because the `done` wake's immediacy
// was keyed on whether anyone was WATCHING the pane (tmux attached-client focus), not
// on whether HQ was AWAITING it — so a `gtmux send`-driven completion in an attended
// pane was deferred to the periodic tick and HQ missed the finish. A pane marked
// awaited fires its next completion wake immediately regardless of attended status,
// then the marker is cleared (a one-shot per dispatch). Backed by an `awaited/<pane>`
// state marker, mirroring the `waiting/<pane>` marker's lifecycle.

// MarkAwaited records that HQ is awaiting pane's completion. Called by the dispatch
// paths (spawn on a delivered dispatch, gtmux send on a landed delivery).
func MarkAwaited(pane string) {
	if pane == "" {
		return
	}
	_ = state.WriteMarker(state.AwaitedPath(pane), "1")
}

// IsAwaited reports whether HQ is awaiting pane's completion.
func IsAwaited(pane string) bool {
	return pane != "" && state.Exists(state.AwaitedPath(pane))
}

// ClearAwaited drops the awaited marker — the await is satisfied (the completion wake
// fired) or moot (the pane went away). A no-op when unmarked.
func ClearAwaited(pane string) {
	if pane == "" {
		return
	}
	state.Remove(state.AwaitedPath(pane))
}
