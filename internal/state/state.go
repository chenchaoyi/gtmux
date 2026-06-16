// Package state centralizes gtmux's on-disk state contract under
// ~/.local/share/gtmux. These paths and file names are a stable interface:
// the `gtmux hook` producer writes them, `gtmux agents` and `gtmux focus
// --last` read them, and the workspace tooling depends on them — do not rename.
//
//	active/<pane>     marker: a turn is in progress for that tmux pane id
//	waiting/<pane>    marker: that pane is blocked on the user (permission/approval)
//	last-finished     the pane id of the most-recently-finished agent turn
//	notify-icon.png   cached agent icon, used as the notification's thumbnail
//	notify/<id>.json  queued desktop-notification requests; the menu-bar app
//	                  drains this dir and posts native banners, then deletes them
package state

import (
	"os"
	"path/filepath"
	"strings"
)

// Dir is ~/.local/share/gtmux (the root of the state contract).
func Dir() string { return filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux") }

// ActivePath is the in-progress marker file for a pane.
func ActivePath(pane string) string { return filepath.Join(Dir(), "active", pane) }

// WaitingDir is the directory of per-pane "blocked on the user" markers.
func WaitingDir() string { return filepath.Join(Dir(), "waiting") }

// WaitingPath is the "blocked on the user" marker file for a pane.
func WaitingPath(pane string) string { return filepath.Join(WaitingDir(), pane) }

// LastFinishedPath holds the pane id of the most-recently-finished turn.
func LastFinishedPath() string { return filepath.Join(Dir(), "last-finished") }

// IconPath is the cached agent icon used as the notification's thumbnail.
func IconPath() string { return filepath.Join(Dir(), "notify-icon.png") }

// NotifyDir is the queue the hook writes notification requests into and the
// menu-bar app drains. It's the delivery channel that replaced terminal-notifier.
func NotifyDir() string { return filepath.Join(Dir(), "notify") }

// Exists reports whether path exists.
func Exists(path string) bool { _, err := os.Stat(path); return err == nil }

// Touch creates path as an empty marker file (its parent dir is created as
// needed). An existing marker is left as-is — only presence matters.
func Touch(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// Remove deletes path, ignoring a missing file.
func Remove(path string) { _ = os.Remove(path) }

// WriteLastFinished records pane as the most-recently-finished turn.
func WriteLastFinished(pane string) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(LastFinishedPath(), []byte(pane+"\n"), 0o644)
}

// ReadLastFinished returns the trimmed pane id from last-finished ("" if none).
func ReadLastFinished() string {
	b, err := os.ReadFile(LastFinishedPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// WaitingSet returns the set of pane ids currently marked waiting.
func WaitingSet() map[string]bool {
	m := map[string]bool{}
	entries, _ := os.ReadDir(WaitingDir())
	for _, e := range entries {
		if !e.IsDir() {
			m[e.Name()] = true
		}
	}
	return m
}
