// Package terminal abstracts the host terminal gtmux drives — the "remote" side
// (focus a tab, spawn tabs, open a window, check what you're viewing) — so gtmux
// isn't hardwired to Ghostty. The "radar" side (agents/overview) stays
// terminal-agnostic and does NOT use this.
//
// Detection of WHICH terminal hosts tmux is a later slice; for now Active()
// always resolves to Ghostty, making the extraction a behavior-preserving
// refactor.
package terminal

import "github.com/chenchaoyi/gtmux/internal/ghostty"

// Terminal is everything terminal-specific gtmux needs. Drivers match a tmux
// session's tab by its title "#S — #W" (tmux set-titles), so that must be on.
type Terminal interface {
	Name() string
	// FocusTab brings the tab for tmux session `session` to the front.
	// Returns "ok"/"notfound" or "" with a non-nil error on failure.
	FocusTab(session string) (string, error)
	// IsViewing reports whether that session's tab is already frontmost
	// (used to suppress a notification you don't need).
	IsViewing(session string) bool
	// OpenWindow opens a new terminal window running a shell command.
	OpenWindow(command string) (string, error)
	// SpawnTabs opens one tab per session (each attaching). dryRun returns the
	// script/plan without executing it.
	SpawnTabs(sessions []string, dryRun bool) (string, error)
}

// registry maps a driver name (see detect.go) to its impl. New terminals are
// registered here; detection resolves the name, this maps it to the driver.
var registry = map[string]Terminal{
	"ghostty": ghostty.Driver{},
	"iterm2":  iterm2{},
}

// fallback is used when the detected terminal has no driver yet (so gtmux on
// Ghostty keeps working unchanged while other drivers are added).
var fallback Terminal = ghostty.Driver{}

// Active returns the terminal driver gtmux should drive — resolved from the host
// terminal (override / this process's env / the tmux client's ancestry), or the
// fallback when the detected terminal isn't supported yet.
func Active() Terminal {
	if t, ok := registry[resolveName()]; ok {
		return t
	}
	return fallback
}
