package app

import (
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
)

// cmdSaveTabOrder records the live terminal tab→session order so `gtmux restore`
// can replay it (instead of tmux's alphabetical `list-sessions` order). The
// always-on menu-bar app calls this on a slow timer. Silent and best-effort.
func cmdSaveTabOrder(args []string) int {
	next := terminal.Active().TabOrder()
	if shrinksTabOrder(next, state.LoadTabOrder()) {
		// A degraded snapshot (classically: right after a reboot only a bootstrap
		// 'main' tab is up) is about to wipe a richer recorded order. Don't — keep
		// the real order around for `gtmux restore` to replay. This is the tab-order
		// analogue of sanitizeLast guarding resurrect's `last`.
		return 0
	}
	_ = state.SaveTabOrder(next)
	return 0
}

// shrinksTabOrder reports whether `next` is a proper subset of `prev`: strictly
// fewer entries, every one of them already known. That's a transient/degraded
// snapshot (post-reboot bootstrap, or an AppleScript read that returned nothing)
// — not a real reorder — so the caller keeps the existing record instead. A `next`
// that introduces ANY new session is a genuine change and is allowed through.
func shrinksTabOrder(next, prev []string) bool {
	if len(prev) == 0 || len(next) >= len(prev) {
		return false
	}
	have := make(map[string]bool, len(prev))
	for _, s := range prev {
		have[s] = true
	}
	for _, s := range next {
		if !have[s] {
			return false
		}
	}
	return true
}

// orderByTabOrder reorders `sessions` to follow `order` (the recorded tab order):
// sessions present in `order` come first, in that order; any remaining sessions
// (not in the record — e.g. created since) keep their original relative order at
// the end. With no record, the input is returned unchanged.
func orderByTabOrder(sessions, order []string) []string {
	if len(order) == 0 {
		return sessions
	}
	have := map[string]bool{}
	for _, s := range sessions {
		have[s] = true
	}
	used := map[string]bool{}
	out := make([]string, 0, len(sessions))
	for _, s := range order {
		if have[s] && !used[s] {
			out = append(out, s)
			used[s] = true
		}
	}
	for _, s := range sessions {
		if !used[s] {
			out = append(out, s)
			used[s] = true
		}
	}
	return out
}
