package app

import (
	"github.com/chenchaoyi/gtmux/internal/servermode"
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func platformTerminalChecks() []dcheck { return []dcheck{rowTerminal(), rowOtherTerminals()} }
func platformSleepChecks() []dcheck    { return sleepSettingChecks() }

func ensureCloudflared(yes bool) string  { return ensureCloudflaredMac(yes) }
func (s *fixState) stepCloudflared() int { return s.stepCloudflaredMac() }
func tmuxInstallHint() string            { return "brew install tmux" }

func prepareHookDesktop() {
	// Retire any legacy one-shot click target from an older install.
	if _, err := os.Stat(legacyFocusAppPath()); err == nil {
		runQuiet(lsregister, "-u", legacyFocusAppPath())
		_ = os.RemoveAll(legacyFocusAppPath())
	}

	// 1. Cache the Claude icon (best-effort).
	if cacheClaudeIcon() {
		i18n.Say("✓ cached Claude icon for notifications", "✓ 已缓存 Claude 通知图标")
	}

}

func platformServiceChecks() []dcheck { return nil }

// sleepSettingChecks reports on the system sleep-disable setting. It is SILENT on a
// machine that has never had it touched, so users who don't use server mode see no
// extra rows (the same courtesy rowCodexHook extends to non-Codex users).
//
// It exists because this state is otherwise invisible: the setting survives reboot
// and `pmset`'s reporting commands never mention it, so a Mac left unable to sleep
// stays that way silently — for months, if nobody thinks to look. Making it legible
// is the whole point.
//
// gtmux reverts only what gtmux owns. An unstamped setting is somebody else's
// deliberate choice and is reported with the manual command, never changed.
func sleepSettingChecks() []dcheck {
	stale := true
	if rec, ok := servermode.LoadState(); ok {
		stale = rec.Stale(time.Now())
	}
	return sleepChecksFor(servermode.Current(), stale)
}
