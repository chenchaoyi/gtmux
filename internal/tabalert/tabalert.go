// Package tabalert marks the TERMINAL TAB of a session that has an agent waiting on you.
//
// THE PROBLEM IT SOLVES. Nine tmux sessions, each attached in its own terminal tab, all
// titled alike: "HQ — hq", "Pica — sat-monitor", "Diting — Diting Mob…". Somewhere in
// there an agent is blocked on a decision, and the only way to find out which is to visit
// each tab. The information gtmux has is not the problem; its PLACEMENT is. The eye
// scans that tab strip constantly and it carries nothing.
//
// WHY THE TAB AND NOT TMUX'S STATUS BAR. tmux's status line only lists the windows of the
// CURRENT session, and with one session per tab it is never in view when the question is
// "which of my nine". The tab strip is what gets scanned, so the tab is what must speak.
//
// WHY THIS IS TERMINAL-AGNOSTIC. tmux renders `set-titles-string` and pushes the result to
// the terminal as an ordinary title escape; the terminal just displays it. Verified live
// on Ghostty before this was written. A per-terminal tab COLOUR would be a different,
// driver-specific capability (internal/terminal) and is deliberately not attempted here:
// the glyph already answers "where do I go", and it answers it everywhere.
//
// WHY GTMUX AND NOT HQ. Rewriting a title as state changes is a mechanical, per-second
// job. Handing it to an LLM would make it slow, expensive, drift-prone, and absent
// whenever HQ is not running. The supervisor is not in this loop at all.
//
// ONLY WAITING MARKS. `working` and `idle` never do. A marker that appears on most tabs
// most of the time is a marker nobody reads — the same reason the commander's colour rule
// keeps red for decisions only. waiting 响, idle 静.
package tabalert

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// SessionOption is the per-session tmux user option the title format interpolates.
const SessionOption = "@gtmux_alert"

// Marker is what a marked tab shows before its title. Shape, not colour: colour in a tab
// is a per-terminal capability, and this must work on every terminal.
const Marker = "● "

// titlesOption is the tmux option that decides what a terminal tab is titled.
const titlesOption = "set-titles-string"

// ownedPrefix is what gtmux prepends when it takes over the title format. Its presence is
// how a later run knows the format is still the one gtmux installed.
const ownedPrefix = "#{" + SessionOption + "}"

// savedPath remembers the user's ORIGINAL title format, so disabling restores exactly
// what was there rather than a guess at a default.
func savedPath() string { return filepath.Join(state.Dir(), "tabalert-saved-titles") }

// Enabled reports whether gtmux currently owns the title format.
func Enabled() bool {
	return strings.HasPrefix(currentFormat(), ownedPrefix)
}

func currentFormat() string {
	return strings.TrimSpace(tmux.ShowGlobalOption(titlesOption))
}

// Enable takes over the title format, PREPENDING the marker to whatever the user already
// had rather than replacing it. Their format is their business; this only asks for a few
// characters in front of it.
func Enable() error {
	cur := currentFormat()
	if strings.HasPrefix(cur, ownedPrefix) {
		return nil // already ours — idempotent
	}
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(savedPath(), []byte(cur), 0o644); err != nil {
		return err
	}
	return tmux.SetGlobalOption(titlesOption, ownedPrefix+cur)
}

// Disable restores the saved format and clears every session's marker.
//
// It refuses to restore over a format that is NOT the one gtmux installed: the user may
// have changed it since, and silently overwriting their edit with a stale snapshot is
// worse than leaving a prefix they can delete themselves.
func Disable() (restored bool, err error) {
	cur := currentFormat()
	if !strings.HasPrefix(cur, ownedPrefix) {
		return false, nil // not ours (or the user rewrote it) — do not touch
	}
	saved, rerr := os.ReadFile(savedPath())
	if rerr != nil {
		// No snapshot: strip our prefix rather than inventing a default.
		saved = []byte(strings.TrimPrefix(cur, ownedPrefix))
	}
	if err := tmux.SetGlobalOption(titlesOption, string(saved)); err != nil {
		return false, err
	}
	for _, s := range tmux.SessionNames() {
		_ = tmux.UnsetSessionOption(s, SessionOption)
	}
	_ = os.Remove(savedPath())
	return true, nil
}

// Reconcile sets or clears every session's marker from the waiting markers on disk.
//
// It reads STATE FILES, not the screen: a waiting marker is written by the agent's own
// hook event, which is the same evidence every other part of gtmux trusts for "is someone
// blocked". It is a no-op when the feature is off, so the tick can call it unconditionally.
func Reconcile() {
	if !Enabled() {
		return
	}
	waiting := waitingSessions()
	for _, s := range tmux.SessionNames() {
		want := ""
		if waiting[s] {
			want = Marker
		}
		// Only write on a CHANGE: `set-option` is cheap but this runs on a tick, and a
		// tmux option write is observable (it redraws the title), so a needless one is a
		// needless repaint on nine terminals.
		if strings.TrimSpace(tmux.ShowSessionOption(s, SessionOption)) != strings.TrimSpace(want) {
			_ = tmux.SetSessionOption(s, SessionOption, want)
		}
	}
}

// waitingSessions maps session name → "has at least one pane waiting on the user".
func waitingSessions() map[string]bool {
	out := map[string]bool{}
	ents, err := os.ReadDir(state.WaitingDir())
	if err != nil {
		return out
	}
	for _, e := range ents {
		pane := e.Name()
		if !strings.HasPrefix(pane, "%") {
			continue
		}
		if s := strings.TrimSpace(tmux.Display(pane, "#{session_name}")); s != "" {
			out[s] = true
		}
	}
	return out
}
