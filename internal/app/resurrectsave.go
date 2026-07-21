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
	// backstopSaveStaleAfter: how old the last resurrect save may get before gtmux serve
	// saves ITSELF. Short enough that a reboot loses little; long enough that when
	// continuum IS autosaving (every few min) the backstop never fires.
	backstopSaveStaleAfter = 10 * time.Minute
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
	cmd := exec.CommandContext(ctx, "bash", script)
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
// saves ONLY when a tmux server is up AND the last save is stale — so when continuum is
// healthy (saving every few min) it's a no-op, and when continuum is disarmed it keeps
// the save fresh. The save runs in its own goroutine (a slow save.sh must not stall the
// tick) under a single-flight guard.
func maybeBackstopSave() {
	if !tmux.ServerUp() {
		return
	}
	if !saveIsStale(resurrectLastSave(), time.Now(), backstopSaveStaleAfter) {
		return
	}
	if !backstopSaving.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer backstopSaving.Store(false)
		driveResurrectSave(resurrectSaveScript())
	}()
}

// statusRightHasContinuumTrigger reports whether a tmux status-right value carries
// tmux-continuum's autosave trigger (`…/continuum_save.sh`). Without it, continuum never
// autosaves — the silent misconfiguration `gtmux doctor` flags.
func statusRightHasContinuumTrigger(sr string) bool {
	return strings.Contains(sr, "continuum_save")
}
