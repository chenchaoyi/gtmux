package app

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Claude Code has two renderers. The classic one draws in the normal screen, so tmux
// keeps its scrollback; the fullscreen one ("tui": "fullscreen", or CLAUDE_CODE_NO_FLICKER)
// runs in the ALTERNATE screen and virtualizes its own history, so tmux holds one screen
// and nothing more. Everything gtmux reads off a pane assumes the first: the phone's
// terminal history, the browser mirror, and the screen reads tuned to the classic layout.
//
// Claude picks fullscreen by itself on a fresh install when "tui" is unset. On 2026-10-03
// a reinstall reset settings.json and every new Claude pane came up in the alternate screen
// with 0–19 lines of tmux history; the phone's terminal "lost" its scrollback overnight and
// nothing in gtmux said why. This row says why, and `--fix` pins the classic renderer.

// claudeVersionCmd matches Claude Code's foreground command when it reports its version
// instead of its name — the same shape the radar accepts (internal/radar/agents.go).
var claudeVersionCmd = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func isClaudeCommand(cmd string) bool {
	return agents.CommandKeys()[cmd] == "claude" || claudeVersionCmd.MatchString(cmd)
}

// claudeTUIFacts is what the row is decided from. Kept separate from reading the machine
// so every case is testable without tmux or a home directory.
type claudeTUIFacts struct {
	Setting  string // settings.json "tui": "default", "fullscreen", or "" when unset
	Forced   string // an env var forcing a renderer: "fullscreen", "default", or ""
	ForcedBy string // which env var did it, for the message
	AltPanes int    // Claude panes in the alternate screen right now
	Panes    int    // Claude panes in total
}

// readClaudeTUIFacts reads the settings file, the env Claude would see, and the panes.
// A variable so tests can substitute the machine.
var readClaudeTUIFacts = func() claudeTUIFacts {
	var f claudeTUIFacts
	m, err := loadJSONObject(claudeSettingsPath())
	if err == nil {
		f.Setting = asString(m["tui"])
	}
	env := map[string]string{}
	if err == nil {
		for k, v := range asObject(m["env"]) {
			env[k] = fmt.Sprint(v)
		}
	}
	// The settings file's env block is what Claude applies; doctor's own environment
	// stands in for the shell a session was launched from.
	get := func(k string) string {
		if v, ok := env[k]; ok {
			return v
		}
		return os.Getenv(k)
	}
	truthy := func(v string) bool { v = strings.ToLower(strings.TrimSpace(v)); return v == "1" || v == "true" }
	switch {
	case truthy(get("CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN")):
		f.Forced, f.ForcedBy = "default", "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN"
	case truthy(get("CLAUDE_CODE_NO_FLICKER")):
		f.Forced, f.ForcedBy = "fullscreen", "CLAUDE_CODE_NO_FLICKER"
	}
	f.AltPanes, f.Panes = claudeAltScreenPanes()
	return f
}

// claudeAltScreenPanes counts Claude panes, and those in the alternate screen. A variable
// so tests do not read the machine's real tmux.
var claudeAltScreenPanes = func() (alt, total int) {
	out, err := tmux.Run("list-panes", "-a", "-F", "#{pane_current_command}\t#{alternate_on}")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(out, "\n") {
		cmd, on, ok := strings.Cut(line, "\t")
		if !ok || !isClaudeCommand(cmd) {
			continue
		}
		total++
		if on == "1" {
			alt++
		}
	}
	return alt, total
}

// claudeTUIPinnable: --fix may write "tui": "default". Only with evidence of fullscreen,
// and never over an explicit env choice either way.
func claudeTUIPinnable(f claudeTUIFacts) bool {
	if f.Forced != "" || f.Setting == "default" {
		return false
	}
	return f.Setting == "fullscreen" || f.AltPanes > 0
}

func claudeTUIRow(f claudeTUIFacts) dcheck {
	label := i18n.Tr("Claude Code renderer", "Claude Code 渲染器")
	why := i18n.Tr("fullscreen runs in the alternate screen: tmux keeps no scrollback, so the phone's terminal shows one screen",
		"全屏渲染器跑在备用屏幕里，tmux 不留滚动历史，手机终端只能看到一屏")
	switch {
	case f.Forced == "fullscreen":
		return dcheck{stRec, label, i18n.Tr("fullscreen (", "全屏（") + f.ForcedBy + i18n.Tr(")", "）"),
			why + i18n.Tr(". Set by "+f.ForcedBy+"; unset it to get scrollback back", "。由 "+f.ForcedBy+" 强制；去掉它才能恢复滚动历史")}
	case f.Setting == "fullscreen":
		return dcheck{stRec, label, i18n.Tr("fullscreen", "全屏"),
			why + i18n.Tr(". `gtmux doctor --fix` sets \"tui\": \"default\"", "。`gtmux doctor --fix` 会设为 \"tui\": \"default\"")}
	case f.Setting == "" && f.Forced == "" && f.AltPanes > 0:
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("fullscreen (%d of %d sessions)", "全屏（%d/%d 个会话）"), f.AltPanes, f.Panes),
			why + i18n.Tr(". Claude chose it (a fresh install does); `gtmux doctor --fix` pins \"tui\": \"default\"",
				"。没有固定时 Claude 会自己选（新装默认全屏）；`gtmux doctor --fix` 会固定为 \"tui\": \"default\"")}
	case f.AltPanes > 0:
		// Pinned (or forced) to classic, yet some panes are still in the alternate screen:
		// sessions started before the setting. gtmux never types into a session to fix it.
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("classic · %d session(s) still fullscreen", "经典 · 仍有 %d 个会话是全屏"), f.AltPanes),
			i18n.Tr("they started before the setting: restart them, or type /tui default in each (it resumes the conversation)",
				"它们是设置之前启动的：重启，或在各会话里输入 /tui default（会恢复原对话）")}
	case f.Setting == "default" || f.Forced == "default":
		return dcheck{stOK, label, i18n.Tr("classic", "经典"), i18n.Tr("tmux keeps scrollback", "tmux 保留滚动历史")}
	default:
		return dcheck{stOK, label, i18n.Tr("classic (not pinned)", "经典（未固定）"),
			i18n.Tr("a fresh install or update may switch to fullscreen; this row will say so",
				"新装或升级可能切到全屏，届时这一行会提示")}
	}
}

func rowClaudeTUI() dcheck { return claudeTUIRow(readClaudeTUIFacts()) }

// stepClaudeTUI pins the classic renderer. Running sessions keep theirs until restarted;
// gtmux does not type /tui into them, and scrollback a fullscreen session never gave tmux
// cannot be recovered — the step says both rather than implying either.
func (s *fixState) stepClaudeTUI() int {
	f := readClaudeTUIFacts()
	if !claudeTUIPinnable(f) {
		return 0
	}
	detail := i18n.Tr(
		"  Pin Claude Code's classic renderer (\"tui\": \"default\" in "+tildeify(claudeSettingsPath())+").\n  Fullscreen runs in the alternate screen, where tmux keeps no scrollback. Backed up first;\n  nothing else in the file changes.",
		"  把 Claude Code 固定为经典渲染器（在 "+tildeify(claudeSettingsPath())+" 写入 \"tui\": \"default\"）。\n  全屏渲染器跑在备用屏幕里，tmux 不留滚动历史。会先备份，文件其他内容不变。")
	if !s.ask(i18n.Tr("Claude Code renderer", "Claude Code 渲染器"), detail) {
		return 0
	}
	if err := pinClaudeTUIDefault(claudeSettingsPath()); err != nil {
		i18n.Sae("  ✗ failed: "+err.Error(), "  ✗ 失败："+err.Error())
		s.rc = 1
		return 0
	}
	i18n.Say("  ✓ pinned. Sessions already running stay fullscreen until restarted (or /tui default in each); history they never gave tmux cannot be recovered",
		"  ✓ 已固定。已经在跑的会话要重启（或各自输入 /tui default）才会切换；全屏期间 tmux 没收到的历史无法找回")
	return 1
}

// pinClaudeTUIDefault sets "tui": "default", leaving every other key as it was.
func pinClaudeTUIDefault(path string) error {
	m, err := loadJSONObject(path)
	if err != nil {
		return err
	}
	backupFile(path)
	m["tui"] = "default"
	return writeJSONObject(path, m)
}
