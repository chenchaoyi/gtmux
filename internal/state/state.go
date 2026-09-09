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
func Dir() string { return filepath.Join(home(), ".local", "share", "gtmux") }

// home is the directory every gtmux path hangs off, and under `go test` it REFUSES to be
// the real one.
//
// Five times between 2026-09-08 and 09-09 a test overwrote the operator's live situation
// board — and every one of those tests had carefully redirected `XDG_CONFIG_HOME` first.
// Nothing here reads that variable. The tests were not careless; the resolver ignored the
// override that looks like the right one, so each round of "make the test write to a temp
// dir" fixed a symptom and the sixth failure was already on its way.
//
// So the real path is now unreachable from a test rather than discouraged. The guard
// belongs here and not on the board writer: the knowledge base, the event log and the
// caches hang off this too, and a guard per writer is a list somebody will forget to add
// to.
//
// That was written calling this "the one chokepoint every gtmux path goes through". It
// was not — twenty-three files read $HOME on their own, including the ones that install
// hooks into ~/.claude/settings.json and write serve's device roster and the pairing
// credentials, so the guard covered the two paths its author had in mind and left the
// rest open. It IS the one chokepoint now, and check-design.sh fails the build on a new
// resolver rather than leaving that a claim in a comment.
//
// It PANICS rather than returning an error. A test that reaches for the real home has a
// bug in the test, and the message says exactly what to do about it; degrading quietly is
// how this went unnoticed five times.
func home() string {
	h := os.Getenv("HOME")
	if underTest() && !disposable(h) {
		panic("gtmux: a test tried to resolve the REAL home (" + h + ").\n" +
			"  Redirect it first:  t.Setenv(\"HOME\", t.TempDir())\n" +
			"  XDG_CONFIG_HOME and XDG_DATA_HOME are NOT read here — setting them does nothing.")
	}
	return h
}

// Home is the same directory, exported for the gtmux paths that do NOT hang off
// Dir() or HQHome() — the user config file, the serve rosters, the hook installs —
// so they resolve through this guard too instead of reading $HOME on their own.
func Home() string { return home() }

// underTest reports whether this process is a `go test` binary. Checked by argv rather
// than by importing `testing`, which would link the test framework and its flags into the
// shipped CLI.
func underTest() bool {
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/_test/") {
		return true
	}
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-test.") {
			return true
		}
	}
	return false
}

// disposable reports whether a home is somewhere a test may write: under one of the
// throwaway roots below.
func disposable(h string) bool {
	if h == "" {
		return false
	}
	for _, root := range tempRoots() {
		if under(h, root) {
			return true
		}
	}
	return false
}

// tempRoots are the directories a test's throwaway HOME may live under.
//
// Two of them, not one. `t.TempDir()` puts a directory under os.TempDir(), which is the
// obvious root — but a dozen tmux tests in this repo deliberately do NOT use it: a unix
// socket path caps near 104 bytes, and on macOS os.TempDir() is already a ~50-character
// /var/folders/… path, so they take a short `os.MkdirTemp("/tmp", "gtx")` instead and
// point HOME at that. Those tests are properly isolated; a guard that calls their home
// "the real one" fires on the wrong people, and the message it prints ("redirect it
// first") is advice they already took.
//
// On Linux the two roots coincide, which is why CI would never have shown this.
func tempRoots() []string {
	roots := []string{filepath.Clean(os.TempDir()), "/tmp"}
	// /tmp is a symlink to /private/tmp on macOS, so a path may be written in either
	// spelling. List both so they compare equal.
	for _, r := range append([]string{}, roots...) {
		if e, err := filepath.EvalSymlinks(r); err == nil && e != r {
			roots = append(roots, e)
		}
	}
	return roots
}

// under reports whether path is root or sits inside it, comparing the path as written
// and, if it exists, as symlinks resolve it.
func under(path, root string) bool {
	c := filepath.Clean(path)
	if c == root || strings.HasPrefix(c, root+string(filepath.Separator)) {
		return true
	}
	e, err := filepath.EvalSymlinks(c)
	if err != nil {
		return false
	}
	return e == root || strings.HasPrefix(e, root+string(filepath.Separator))
}

// HQHome is the supervisor (中控) agent's persistent working directory — where
// `gtmux hq` runs the agent and seeds its instructions file, and the cwd the
// radar/hook use to RECOGNIZE a supervisor pane (`role:"supervisor"`, the waiting
// nudge). Under ~/.config (user-editable instructions + accumulated knowledge,
// not machine state). Shared here because both `internal/app` (hq/agents) and
// `internal/hook` (nudge) need the same path without an import cycle.
func HQHome() string { return filepath.Join(home(), ".config", "gtmux", "hq") }

// WatchedDir is the directory of per-pane "watch this pane" markers — a user has
// opted a PLAIN (non-agent) pane onto the radar (tiered-pane-control). A marker's
// presence is the whole signal; it is never auto-created, and is reaped when its
// pane closes (see ReapOrphanTurnMarkers).
func WatchedDir() string { return filepath.Join(Dir(), "watched") }

// WatchedPath is the "watch this pane" marker for a pane id.
func WatchedPath(pane string) string { return filepath.Join(WatchedDir(), pane) }

// IsWatched reports whether a pane has been opted onto the radar.
func IsWatched(pane string) bool { return Exists(WatchedPath(pane)) }

// WatchedPanes lists the pane ids currently watched (order unspecified).
func WatchedPanes() []string {
	entries, err := os.ReadDir(WatchedDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// ActiveDir is the directory of per-pane "agent is mid-turn" markers.
func ActiveDir() string { return filepath.Join(Dir(), "active") }

// ActivePath is the in-progress marker file for a pane.
func ActivePath(pane string) string { return filepath.Join(ActiveDir(), pane) }

// ReapOrphanTurnMarkers deletes the per-pane TURN-STATE markers (active / waiting /
// finished) for panes NOT in `live` — panes tmux no longer has (an agent exited and
// its pane was closed, or the whole session went away). These markers are keyed by
// pane id, so a closed pane leaves them behind as cruft that can feed phantom state
// into surfaces that look a pane up by id. Resume records are deliberately NOT
// touched — they must survive a closed pane so `restore` can bring the conversation
// back after a reboot (when EVERY pane is momentarily gone). Best-effort; returns the
// number of marker files removed.
func ReapOrphanTurnMarkers(live map[string]bool) int {
	removed := 0
	// watched/ is included: a watched plain pane is dropped from the radar the moment
	// its pane closes (tiered-pane-control), the same lifecycle as the turn markers.
	for _, dir := range []string{ActiveDir(), WaitingDir(), FinishedDir(), WatchedDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || live[e.Name()] {
				continue
			}
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				removed++
			}
		}
	}
	return removed
}

// panePrefix is what every tmux pane id starts with.
const panePrefix = "%"

// IsPaneID reports whether `name` has the shape of a tmux pane id ("%" then digits).
// It is the safety catch for every pane-keyed sweep: the directories below hold a few
// files that are NOT keyed by pane (hqwake's `consumed-seq`, `unread-state`,
// `selfrotate-state`…), and a sweep that deleted those would take out HQ's consumption
// watermark along with the cruft.
func IsPaneID(name string) bool {
	if !strings.HasPrefix(name, panePrefix) || len(name) < 2 {
		return false
	}
	for _, r := range name[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// panePlainDirs are the state directories whose every pane-keyed entry is named by the
// bare pane id. They are swept together because they share one lifecycle: the pane.
var panePlainDirs = []string{
	"active", "waiting", "finished", "watched", // turn state (also ReapOrphanTurnMarkers')
	"enrolled",     // "HQ has met this pane" — the new-session dedup
	"goal",         // the pane's last user-direct goal, read back by the done wake
	"goalchanged",  // the goal-changed wake's dedup record
	"awaited",      // "HQ awaits this pane's completion"
	"bg",           // background-work-still-running modifier
	"usagewarn",    // the amber burn modifier
	"frame", "cpu", // the hook-free working/idle samplers
	"watchdog", // "already escalated this stuck episode"
}

// paneJSONDirs are pane-keyed directories whose entries carry a `.json` suffix.
var paneJSONDirs = []string{"sends"} // the re-send interlock's last-payload record

// hqwakePaneFamilies are the pane-keyed file FAMILIES inside the (otherwise
// shared) hqwake directory: `<prefix><pane>`.
var hqwakePaneFamilies = []string{"done-last-", "resolved-claim-", "resolved-track-"}

// ReapDeadPaneState deletes every pane-keyed state file whose pane tmux no longer has.
//
// # Why this has to exist
//
// A tmux pane id is a SEQUENCE NUMBER on the server, not an identity. Restart the server
// — a reboot, a crash, `kill-server` — and it starts issuing from %1 again, handing your
// old numbers to entirely different panes. gtmux keys a dozen state directories by that
// number and, until this, only ever cleaned the three turn-state ones. Everything else
// accumulated: measured on 2026-08-18, right after a reboot, 50 of 52 `enrolled` records,
// 27 of 29 `goal`s, 31 of 32 `sends` and 93 of 103 `hqwake` files named panes that had
// not existed for up to two weeks — and the live panes had inherited those very numbers.
//
// That is not merely litter, it is state CROSS-WIRED between unrelated sessions. Two
// measured consequences from that one reboot: `goal/%11` still held what the commander
// had told a session about a completely different project, so anything reading that
// pane's goal attributed one team's words to another's; and a dispatch record from two
// weeks earlier, still `delivered:false`, made gtmux screen-scrape the pane that had
// inherited its number, misread a startup menu, and raise a fabricated alarm about a
// session it had never dispatched anything to.
//
// # What it deliberately does NOT touch
//
//   - `resume/` — keyed by LOCATOR (session:window.pane), not pane id, and it must
//     survive a reboot: at the moment restore runs, every pane is gone, and these
//     records are the only memory of which conversation belongs where.
//   - `usage/` and `native/` — keyed by the agent's conversation/session id. A dead pane
//     says nothing about a conversation's usefulness.
//   - anything whose name is not a pane id (see IsPaneID).
//
// `live` is the set of pane ids tmux currently reports. An EMPTY set is refused
// outright: a transient failed `list-panes` would otherwise read as "no panes exist"
// and reap the entire fleet's state. Best-effort; returns how many files it removed.
func ReapDeadPaneState(live map[string]bool) int {
	if len(live) == 0 {
		return 0
	}
	base := Dir()
	removed := 0
	sweep := func(dir, prefix, suffix string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
				continue
			}
			pane := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
			if !IsPaneID(pane) || live[pane] {
				continue
			}
			if os.Remove(filepath.Join(dir, name)) == nil {
				removed++
			}
		}
	}
	for _, d := range panePlainDirs {
		sweep(filepath.Join(base, d), "", "")
	}
	for _, d := range paneJSONDirs {
		sweep(filepath.Join(base, d), "", ".json")
	}
	for _, fam := range hqwakePaneFamilies {
		sweep(filepath.Join(base, "hqwake"), fam, "")
	}
	return removed
}

// WaitingDir is the directory of per-pane "blocked on the user" markers.
func WaitingDir() string { return filepath.Join(Dir(), "waiting") }

// WaitingPath is the "blocked on the user" marker file for a pane.
func WaitingPath(pane string) string { return filepath.Join(WaitingDir(), pane) }

// AwaitedDir is the directory of per-pane "HQ is awaiting this dispatch's completion"
// markers (done-wake-keyed-on-awaited). A pane HQ dispatched work to (spawn or send)
// gets a marker so its NEXT completion wakes HQ immediately even when the pane is
// attended — the case a plain attended-defer would drop.
func AwaitedDir() string { return filepath.Join(Dir(), "awaited") }

// AwaitedPath is the "HQ awaits this pane's completion" marker file for a pane.
func AwaitedPath(pane string) string { return filepath.Join(AwaitedDir(), pane) }

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

// UsageWarnDir is the directory of per-pane usage-warning markers (usage-watch):
// content = the compact warn string (e.g. "ctx 86%"). Written/cleared by the
// hook's evaluation; the radar reads it as an amber MODIFIER (like errored/bg).
func UsageWarnDir() string { return filepath.Join(Dir(), "usagewarn") }

// UsageWarnPath is a pane's usage-warning marker.
func UsageWarnPath(pane string) string { return filepath.Join(UsageWarnDir(), pane) }

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

// ReadInt64Marker reads a marker holding a decimal int64 (0 when missing/empty/malformed)
// — the "last-run timestamp / counter" shape that half a dozen sensors and watchdogs
// (selfcheck, distill, disk-hygiene, feed-restart, tally) each hand-rolled as
// strconv.ParseInt(ReadMarker(path), 10, 64).
func ReadInt64Marker(path string) int64 {
	n, _ := strconv.ParseInt(ReadMarker(path), 10, 64)
	return n
}

// WriteInt64Marker writes a decimal int64 to a marker file — the counterpart to
// ReadInt64Marker (was strconv.FormatInt + WriteMarker at each call site).
func WriteInt64Marker(path string, n int64) error {
	return WriteMarker(path, strconv.FormatInt(n, 10))
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
