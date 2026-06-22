package app

import (
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
)

// cmdSaveTabOrder records the live terminal tab→session order so `gtmux restore`
// can replay it (instead of tmux's alphabetical `list-sessions` order). The
// always-on menu-bar app calls this on a slow timer. Silent and best-effort.
func cmdSaveTabOrder(args []string) int {
	_ = state.SaveTabOrder(terminal.Active().TabOrder())
	return 0
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
