package app

import (
	"os"

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
