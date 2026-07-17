// Package hqpane resolves the live HQ (supervisor) pane — the ONE rule every wake
// call site shares.
//
// The bug it fixes (hq-wake-reliability): both call sites compared tmux's
// `#{pane_current_path}` to `state.HQHome()` with a plain string ==. tmux reports the
// PHYSICAL path while HQHome() is built from $HOME, so a single symlink anywhere on
// the way — `~/.config` symlinked into a dotfiles repo is a common setup, and macOS
// aliases /tmp → /private/tmp — made every wake resolve "no HQ" and return silently.
// A `cd` inside the HQ pane did the same. Three criteria now answer the question, and
// a resolve is stamped so a caller can tell "there is no HQ" apart from "an HQ was here
// a minute ago and something is wrong" — the difference between dropping a wake and
// holding it.
package hqpane

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// HomeOption is the pane-level tmux user option `gtmux hq` stamps on the supervisor
// pane at spawn. Its VALUE is the HQ home the pane serves — not a bare "hq" flag —
// which keeps the criterion cwd-proof AND scoped: a tmux server shared by two gtmux
// installs (a different $HOME, a test) resolves each to its own supervisor, and never
// to the other's.
const HomeOption = "@gtmux_hq_home"

// paneFmt lists each pane with everything the criteria need: the stamped home, the
// live cwd, and the directory it started in.
const paneFmt = "#{pane_id}\t#{" + HomeOption + "}\t#{pane_current_path}\t#{pane_start_path}"

// seenStampPath records the last time an HQ pane actually resolved.
func seenStampPath() string { return filepath.Join(state.Dir(), "hqwake", "last-seen-hq") }

// lister is the injectable pane source (real tmux in production, a fixture in tests).
var lister = func() []string { return tmux.Lines("list-panes", "-a", "-F", paneFmt) }

// Find returns the pane id of a live supervisor pane ("" when none).
func Find() string { return resolve() }

// FindOther returns a live supervisor pane that is NOT `about`, and reports whether
// `about` IS the supervisor. The two "" cases must not be confused: `self` means stay
// silent (gtmux never wakes HQ about HQ itself), while a plain "" means "no HQ
// resolved right now" — which a caller may answer by HOLDING the wake (SeenRecently).
func FindOther(about string) (pane string, self bool) {
	pane = resolve()
	if pane != "" && pane == about {
		return "", true
	}
	return pane, false
}

// resolve scans the panes for the HQ home and stamps a hit.
func resolve() string {
	home := normalize(state.HQHome())
	for _, line := range lister() {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 || !isHQ(f, home) {
			continue
		}
		stampSeen()
		return f[0]
	}
	return ""
}

// isHQ reports whether a pane record names the HQ home in ANY of its three fields:
// the stamp `gtmux hq` left (survives a `cd` — the strongest), its current path, or
// the path it started in (an HQ that has since `cd`'d away, on a pane predating the
// stamp). Each is compared symlink-normalized, which is the whole point: tmux reports
// physical paths and HQHome() is built from $HOME.
func isHQ(f []string, home string) bool {
	return normalize(f[1]) == home || normalize(f[2]) == home || normalize(f[3]) == home
}

// normalize resolves a path's symlinks so a logical and a physical spelling of the
// same directory compare equal. A path that cannot be resolved (it was deleted, or
// the process cannot stat it) falls back to its cleaned self — the rule may only ADD
// matches, never remove one.
func normalize(p string) string {
	if p == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(r)
	}
	return filepath.Clean(p)
}

// stampSeen records that an HQ pane resolved just now.
func stampSeen() {
	if err := os.MkdirAll(filepath.Dir(seenStampPath()), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(seenStampPath(), nil, 0o644); err != nil {
		return
	}
	now := time.Now()
	_ = os.Chtimes(seenStampPath(), now, now)
}

// SeenWindow is how long after the last successful resolve a wake is still HELD
// (queued) rather than dropped when no HQ resolves. Long enough to cover an HQ pane
// restarting, a tmux server hiccup, or a resolution bug we have not found yet; short
// enough that a machine which simply stopped running an HQ queues nothing.
const SeenWindow = 2 * time.Hour

// SeenRecently reports whether an HQ pane resolved within SeenWindow. Callers use it
// to decide between HOLDING a wake for a later drain and dropping it: there is no
// point queueing wakes on a machine that runs no supervisor at all.
func SeenRecently() bool {
	fi, err := os.Stat(seenStampPath())
	return err == nil && time.Since(fi.ModTime()) < SeenWindow
}

// Stamp marks a pane as the supervisor for THIS home (`gtmux hq` at spawn), so
// resolution never has to reason about its paths again. Best-effort — the path
// criteria remain the fallback, and they are what a pane spawned before this shipped
// still resolves by.
func Stamp(pane string) {
	if pane == "" {
		return
	}
	_, _ = tmux.Run("set-option", "-p", "-t", pane, HomeOption, state.HQHome())
}

// SeenStampPath is where the last successful resolve is recorded. Exported so the
// hold-versus-drop decision is inspectable (and testable) from the wake call sites.
func SeenStampPath() string { return seenStampPath() }
