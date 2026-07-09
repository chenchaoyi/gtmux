// Package state centralizes gtmux's on-disk state contract under
// ~/.local/share/gtmux. These paths and file names are a stable interface:
// the `gtmux hook` producer writes them, `gtmux agents` and `gtmux focus
// --last` read them, and the workspace tooling depends on them — do not rename.
//
//	active/<pane>     marker: a turn is in progress for that tmux pane id
//	waiting/<pane>    marker: that pane is blocked on the user (permission/approval)
//	finished/<pane>   marker: mtime = when that pane's turn ended (idle duration)
//	bg/<pane>         marker: an idle pane whose turn ended with background work still
//	                  running (content: "<count>\t<label>"); the radar reads it to mark
//	                  the row "background running" — a modifier on idle, never a status
//	native/<session>  record: an agent session sensed OUTSIDE tmux (internal/native)
//	last-finished     the pane id of the most-recently-finished agent turn
//	notify-icon.png   cached agent icon, used as the notification's thumbnail
//	notify/<id>.json  queued desktop-notification requests; the menu-bar app
//	                  drains this dir and posts native banners, then deletes them
package state

import (
	"os"
	"path/filepath"
	"strconv"
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

// FinishedDir is the directory of per-pane "turn finished at" markers.
func FinishedDir() string { return filepath.Join(Dir(), "finished") }

// FinishedPath is the "turn finished at" marker for a pane — its mtime is when
// the agent's turn ended, i.e. how long the pane has been idle. `gtmux agents`
// reads it so an idle session's relative time reflects when it FINISHED, not the
// last time its TUI redrew (a live status line keeps window-activity ticking).
func FinishedPath(pane string) string { return filepath.Join(FinishedDir(), pane) }

// BackgroundDir is the directory of per-pane "background work still running" markers.
func BackgroundDir() string { return filepath.Join(Dir(), "bg") }

// BackgroundPath is the "background work still running" marker for a pane. Its
// content is "<count>\t<label>" — the number of in-flight background tasks the
// agent reported when its turn ended, plus a short label (e.g. the shell command).
func BackgroundPath(pane string) string { return filepath.Join(BackgroundDir(), pane) }

// WriteBackground records that pane's turn ended with count background tasks still
// in flight, plus a short label. A count <= 0 clears the marker instead.
func WriteBackground(pane string, count int, label string) error {
	if count <= 0 {
		Remove(BackgroundPath(pane))
		return nil
	}
	label = strings.ReplaceAll(strings.TrimSpace(label), "\t", " ")
	return WriteMarker(BackgroundPath(pane), strconv.Itoa(count)+"\t"+label)
}

// ReadBackground returns the pane's background-work count and label (0, "" when the
// marker is missing or malformed).
func ReadBackground(pane string) (count int, label string) {
	s := ReadMarker(BackgroundPath(pane))
	if s == "" {
		return 0, ""
	}
	c, rest, found := strings.Cut(s, "\t")
	n, err := strconv.Atoi(strings.TrimSpace(c))
	if err != nil || n <= 0 {
		return 0, ""
	}
	if !found {
		return n, ""
	}
	return n, strings.TrimSpace(rest)
}

// ClearBackground removes a pane's background-work marker.
func ClearBackground(pane string) { Remove(BackgroundPath(pane)) }

// IconPath is the cached agent icon used as the notification's thumbnail.
func IconPath() string { return filepath.Join(Dir(), "notify-icon.png") }

// RemoteClientsPath holds the live remote-viewer roster
// ({"clients":[{name,kind,platform,ip,connectedAt}],"count":N,"at":unix}), written
// by `gtmux serve` on every SSE connect/disconnect + a heartbeat while clients are
// connected; the menu-bar app reads it to show WHO is connected (and treats a stale
// `at` as disconnected). `count` is retained for older readers.
func RemoteClientsPath() string { return filepath.Join(Dir(), "remote-clients.json") }

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

// WriteMarker writes content to a marker file (creating its parent dir). Unlike
// Touch, the marker carries a small payload — e.g. the active turn's agent
// session id, so a late hook from a superseded session can be told apart.
func WriteMarker(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadMarker returns a marker file's trimmed content ("" if missing/empty). An
// empty-but-present marker (from Touch) reads as "".
func ReadMarker(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

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
