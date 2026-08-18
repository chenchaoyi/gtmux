// gtmux's backstop for tmux-resurrect SAVES. restore.go drives the RESTORE; this file
// makes the save gtmux relies on actually stay fresh, instead of trusting tmux-continuum
// to autosave (a custom status-right silently disables continuum's autosave — the save
// then goes stale and a reboot restores an ancient snapshot; see the restore-save-
// reliability change + the restore-reboot-resurrect notes).
package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const (
	// backstopSaveStaleAfter: how long the last resurrect save may sit unchanged before
	// gtmux serve saves ITSELF, when nothing else even claims to be autosaving. Short
	// enough that a reboot loses little.
	backstopSaveStaleAfter = 10 * time.Minute
	// backstopArmedStaleAfter: the same question when tmux-continuum's trigger IS in
	// status-right. Wider, because an armed autosaver deserves the first move — but
	// FINITE, because "armed" is a configuration and this file's freshness is the fact
	// (see backstopStaleAfter).
	backstopArmedStaleAfter = 20 * time.Minute
	// backstopArmedYield: how long to wait, after deciding to step in for an ARMED
	// autosaver, before actually saving — then re-check. The collision that matters is
	// the moment the Mac wakes: the status bar redraws (continuum saves) and serve's tick
	// resumes (gtmux notices the staleness) within the same second or two. Yielding lets
	// continuum's save land and repoint `last`, and the re-check then stands the backstop
	// down instead of running a second save_all over the same files.
	backstopArmedYield = 8 * time.Second
	// restoreWarnStaleAfter: at restore time, a save older than this is a red flag — a
	// healthy setup saves every few minutes, so a day-old save means autosave is dead.
	restoreWarnStaleAfter = 24 * time.Hour
)

// backstopSaving is a single-flight guard so overlapping slow ticks can't launch two
// concurrent save.sh subprocesses.
var backstopSaving atomic.Bool

// saveIsStale reports whether the resurrect save at lastPath is missing, unreadable, or
// older than threshold. os.Stat follows the `last` symlink to the real save file, so the
// mtime is the actual last-save time.
func saveIsStale(lastPath string, now time.Time, threshold time.Duration) bool {
	if lastPath == "" {
		return true
	}
	fi, err := os.Stat(lastPath)
	if err != nil {
		return true
	}
	return now.Sub(fi.ModTime()) >= threshold
}

// saveAge is how long ago the save at lastPath was last written, and whether that is
// knowable at all (a missing path / unreadable file answers no). Separate from
// saveIsStale because doctor wants to SHOW the number, not just compare it.
func saveAge(lastPath string, now time.Time) (time.Duration, bool) {
	if lastPath == "" {
		return 0, false
	}
	fi, err := os.Stat(lastPath)
	if err != nil {
		return 0, false
	}
	return now.Sub(fi.ModTime()), true
}

// saveStalenessWarning returns a localized "your saved layout is N old" line when the
// save at lastPath is older than restoreWarnStaleAfter, else "". Keeps a silently-broken
// autosave from restoring an ancient snapshot without any signal. Pure (mtime + now in,
// string out) so it's unit-testable.
func saveStalenessWarning(lastPath string, now time.Time) string {
	if lastPath == "" {
		return ""
	}
	fi, err := os.Stat(lastPath)
	if err != nil {
		return ""
	}
	age := now.Sub(fi.ModTime())
	if age < restoreWarnStaleAfter {
		return ""
	}
	days := int(age.Hours()) / 24
	return i18n.Tr(
		fmt.Sprintf("⚠ your saved tmux layout is %dd old — autosave looks broken; sessions created since won't restore (run `gtmux doctor`)", days),
		fmt.Sprintf("⚠ 你的 tmux 存档已 %d 天未更新 —— 自动保存疑似坏了;此后新建的 session 无法恢复(运行 `gtmux doctor`)", days))
}

// resurrectSaveScript resolves tmux-resurrect's save.sh (mirrors resurrectRestoreScript).
func resurrectSaveScript() string {
	home := os.Getenv("HOME")
	cands := []string{
		home + "/.tmux/plugins/tmux-resurrect/scripts/save.sh",
		home + "/.config/tmux/plugins/tmux-resurrect/scripts/save.sh",
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		cands = append(cands, xdg+"/tmux/plugins/tmux-resurrect/scripts/save.sh")
	}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// driveResurrectSave runs save.sh as a DIRECT subprocess — never `tmux run-shell`, which
// runs in the server's minimal-PATH env, exits 127, and writes an EMPTY save that
// poisons `last` (the exact self-inflicted failure the restore-reboot-resurrect notes
// warn about). $TMUX + restorePATH mirror driveResurrectRestore; sanitizeLast repairs a
// poisoned pointer afterward as a belt.
func driveResurrectSave(script string) {
	if script == "" {
		return
	}
	socket := tmux.Display("", "#{socket_path}")
	pid := tmux.Display("", "#{pid}")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// "quiet" is REQUIRED, not cosmetic. Without it resurrect's save.sh forks a spinner
	// that writes "Saving..." into tmux's message line and then displays "Tmux
	// environment saved!" — so gtmux's own backstop was painting a flicker across every
	// client on a cadence nobody could account for, while continuum (which does pass
	// quiet) stayed silent. The spinner is also an extra background fork per save.
	argv := resurrectSaveArgs(script)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	env := os.Environ()
	if socket != "" {
		env = append(env, "TMUX="+socket+","+pid+",0")
	}
	env = append(env, "PATH="+restorePATH())
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	restoreLogf("driveResurrectSave: script=%s exit=%v socket=%s\n--- save.sh output ---\n%s--- end ---",
		script, err, socket, string(out))
	sanitizeLast() // never leave a poisoned (empty) `last` behind
}

// maybeBackstopSave is called on the serve slow tick. It backstops tmux-continuum: it
// saves ONLY when a tmux server is up AND the last save has actually gone stale — so
// when the autosave is working (saving every few min) it's a no-op, and when the
// autosave has stopped it keeps the save fresh. The save runs in its own goroutine (a
// slow save.sh must not stall the tick) under a single-flight guard.
func maybeBackstopSave() {
	if !tmux.ServerUp() {
		return
	}
	statusRight := tmuxOpt("status-right")
	if !shouldBackstopSave(statusRight, resurrectLastSave(), time.Now()) {
		return
	}
	if !backstopSaving.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer backstopSaving.Store(false)
		// Yield to an ARMED autosaver before stepping on its toes. See
		// backstopArmedYield: the one moment both savers plausibly fire at once is a
		// wake-from-sleep, and letting continuum's save land first turns that collision
		// into a no-op instead of two concurrent save_all runs.
		if statusRightHasContinuumTrigger(statusRight) {
			time.Sleep(backstopArmedYield)
			if !shouldBackstopSave(statusRight, resurrectLastSave(), time.Now()) {
				restoreLogf("maybeBackstopSave: stood down — the autosaver saved during the %v grace", backstopArmedYield)
				return
			}
		}
		restoreLogf("maybeBackstopSave: save unchanged for >= %v (autosave trigger present=%v) — saving ourselves",
			backstopStaleAfter(statusRight), statusRightHasContinuumTrigger(statusRight))
		driveResurrectSave(resurrectSaveScript())
	}()
}

// resurrectSaveArgs is how resurrect's save script must be invoked.
//
// "quiet" is REQUIRED, not cosmetic. Without it save.sh forks a spinner that writes
// "Saving..." into tmux's message line and then displays "Tmux environment saved!" — so
// gtmux's backstop painted a flicker across every attached client on a cadence nobody
// could account for, while continuum (which does pass quiet) stayed silent. The spinner
// is also an extra background fork per save.
func resurrectSaveArgs(script string) []string {
	return []string{"bash", script, "quiet"}
}

// shouldBackstopSave reports whether gtmux should save the tmux layout ITSELF: the last
// save has gone stale, judged against the grace this setup has earned.
//
// The question used to be "is continuum's trigger in status-right?", and that question
// has a hole big enough to lose a working day through: the trigger only RUNS when tmux
// redraws the status bar, which needs a terminal attached and awake. Written into
// status-right, it looks armed forever; asleep, it saves nothing. On this user's Mac the
// two readings diverged 76 times in three and a half days — gaps up to six hours against
// a configured five-minute interval — and because the trigger was present the whole time,
// the backstop that exists for exactly this never fired ONCE. So the trigger is no longer
// the criterion; the file's mtime is. Configuration is a claim, freshness is the fact.
//
// The old check's lesson is kept, not discarded: two concurrent save_all runs over the
// same files produced paired save files and a truncated pane_contents.tar.gz, and
// corrupting the save is strictly worse than the staleness this guards against. Staleness
// itself is now the interlock — an autosaver that IS running keeps the save fresh, so the
// backstop never wakes up beside it — reinforced by a wider grace when the trigger is
// present (backstopStaleAfter) and a yield-then-recheck before the save actually runs
// (backstopArmedYield).
func shouldBackstopSave(statusRight, lastPath string, now time.Time) bool {
	return saveIsStale(lastPath, now, backstopStaleAfter(statusRight))
}

// backstopStaleAfter is how long a save may sit unchanged before gtmux saves it itself.
// An armed autosaver gets the wider grace — it deserves the first move — but a finite
// one, because "armed" describes the configuration and only the file describes reality.
func backstopStaleAfter(statusRight string) time.Duration {
	if statusRightHasContinuumTrigger(statusRight) {
		return backstopArmedStaleAfter
	}
	return backstopSaveStaleAfter
}

// statusRightHasContinuumTrigger reports whether a tmux status-right value carries
// tmux-continuum's autosave trigger (`…/continuum_save.sh`). Without it, continuum never
// autosaves — the silent misconfiguration `gtmux doctor` flags.
func statusRightHasContinuumTrigger(sr string) bool {
	return continuumTriggerCount(sr) > 0
}

// continuumTriggerCount counts how many autosave triggers a status-right carries.
//
// TWO is a real and silent misconfiguration, and it happens for a specific reason:
// continuum decides whether to inject its trigger by looking for its OWN absolute path.
// A trigger written by hand as `#(~/.tmux/plugins/.../continuum_save.sh)` does not match
// that comparison, so continuum appends a second, absolute-path copy — and every save
// interval then runs the save script twice, forever, with nothing to say so.
//
// gtmux cannot fix continuum's comparison (it is that plugin's own shell code, and gtmux
// never writes status-right — it only reads it). What it CAN do is notice, which is
// exactly what doctor is for. Counting is deliberately path-FORM agnostic: `~` and the
// expanded absolute path are the same script, so both are counted, and two spellings of
// it are still two triggers.
func continuumTriggerCount(sr string) int {
	return strings.Count(sr, "continuum_save")
}
