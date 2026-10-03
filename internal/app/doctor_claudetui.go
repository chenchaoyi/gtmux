package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
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
//
// The rules below follow Claude 2.1.285 as read from its binary: env beats settings, with
// CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN first; CLAUDE_CODE_NO_FLICKER forces fullscreen when
// true AND classic when false; booleans are 1/true/yes/on and 0/false/no/off; and only
// "default" and "fullscreen" are "tui" values — anything else counts as unset.

// claudeVersionCmd matches Claude Code's foreground command when it reports its version
// instead of its name — the same shape the radar accepts (internal/radar/agents.go).
var claudeVersionCmd = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func isClaudeCommand(cmd string) bool {
	return agents.CommandKeys()[cmd] == "claude" || claudeVersionCmd.MatchString(cmd)
}

// claudeEnvBool reads an env value the way Claude does: (value, true) for a recognised
// spelling, (false, false) for anything else, which Claude ignores.
func claudeEnvBool(v string) (val, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

// claudeTUIFacts is what the row is decided from. Kept separate from reading the machine
// so every case is testable without tmux or a home directory.
type claudeTUIFacts struct {
	ReadErr  string // settings.json exists but could not be read or parsed
	Setting  string // settings.json "tui" as written ("" when unset)
	Forced   string // an env var forcing a renderer: "fullscreen", "default", or ""
	ForcedBy string // which env var did it, for the message
	AltPanes int    // Claude panes in the alternate screen right now
	Panes    int    // Claude panes in total
}

// knownSetting is the "tui" value Claude acts on: "default", "fullscreen", or "".
func (f claudeTUIFacts) knownSetting() string {
	if f.Setting == "default" || f.Setting == "fullscreen" {
		return f.Setting
	}
	return ""
}

// readClaudeTUIFacts reads the settings file, the env Claude would see, and the panes.
// A variable so tests can substitute the machine.
var readClaudeTUIFacts = func() claudeTUIFacts {
	var f claudeTUIFacts
	m, err := loadJSONObject(claudeSettingsPath())
	if err != nil {
		// Unreadable or not JSON: nothing can be inferred from it, and saying "unset"
		// would turn a broken file into a pass.
		f.ReadErr = err.Error()
	}
	env := map[string]string{}
	if err == nil {
		f.Setting = asString(m["tui"])
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
	if v, ok := claudeEnvBool(get("CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN")); ok && v {
		f.Forced, f.ForcedBy = "default", "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN"
	} else if v, ok := claudeEnvBool(get("CLAUDE_CODE_NO_FLICKER")); ok {
		if v {
			f.Forced, f.ForcedBy = "fullscreen", "CLAUDE_CODE_NO_FLICKER"
		} else {
			f.Forced, f.ForcedBy = "default", "CLAUDE_CODE_NO_FLICKER"
		}
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
	return countClaudeAltPanes(out)
}

// countClaudeAltPanes parses `list-panes -F '#{pane_current_command}\t#{alternate_on}'`.
func countClaudeAltPanes(out string) (alt, total int) {
	for _, line := range strings.Split(out, "\n") {
		cmd, on, ok := strings.Cut(line, "\t")
		if !ok || !isClaudeCommand(cmd) {
			continue
		}
		total++
		if strings.TrimSpace(on) == "1" {
			alt++
		}
	}
	return alt, total
}

// claudeTUIPinnable: --fix may write "tui": "default". Only with evidence of fullscreen,
// a settings file it could read, and never over an explicit env choice either way.
func claudeTUIPinnable(f claudeTUIFacts) bool {
	if f.ReadErr != "" || f.Forced != "" || f.knownSetting() == "default" {
		return false
	}
	return f.knownSetting() == "fullscreen" || f.AltPanes > 0
}

func claudeTUIRow(f claudeTUIFacts) dcheck {
	label := i18n.Tr("Claude Code renderer", "Claude Code 渲染器")
	why := i18n.Tr("fullscreen runs in the alternate screen: tmux keeps no scrollback, so the phone's terminal shows one screen",
		"全屏渲染器跑在备用屏幕里，tmux 不留滚动历史，手机终端只能看到一屏")
	// Some sessions still fullscreen under a classic setting. The usual reason is that they
	// started before it; a project or managed setting, or the env a session was launched
	// with, can force it too, and doctor sees none of those.
	stillFull := func(value string) dcheck {
		return dcheck{stRec, label, value + fmt.Sprintf(i18n.Tr(" · %d session(s) still fullscreen", " · 仍有 %d 个会话是全屏"), f.AltPanes),
			i18n.Tr("most likely they started before the setting: restart them, or type /tui default in each (it resumes the conversation). A project or managed setting, or the env a session started with, can also force it",
				"多半是设置之前启动的：重启，或在各会话里输入 /tui default（会恢复原对话）。项目级或托管设置、会话启动时的环境变量也可能强制全屏")}
	}
	unknown := ""
	if f.Setting != "" && f.knownSetting() == "" {
		unknown = fmt.Sprintf(i18n.Tr(" (\"tui\": %q is not a value Claude knows; it counts as unset)", "（\"tui\": %q 不是 Claude 认识的值，按未设置处理）"), f.Setting)
	}
	switch {
	case f.ReadErr != "":
		return dcheck{stRec, label, i18n.Tr("settings.json unreadable", "settings.json 读不了"),
			i18n.Tr("cannot tell the renderer: ", "判断不了渲染器：") + f.ReadErr}
	case f.Forced == "fullscreen":
		return dcheck{stRec, label, i18n.Tr("fullscreen (", "全屏（") + f.ForcedBy + i18n.Tr(")", "）"),
			why + i18n.Tr(". Set by "+f.ForcedBy+"; unset it to get scrollback back", "。由 "+f.ForcedBy+" 强制；去掉它才能恢复滚动历史")}
	case f.Forced == "default":
		value := i18n.Tr("classic (", "经典（") + f.ForcedBy + i18n.Tr(")", "）")
		if f.AltPanes > 0 {
			return stillFull(value)
		}
		note := i18n.Tr("tmux keeps scrollback", "tmux 保留滚动历史")
		if f.knownSetting() == "fullscreen" {
			note = i18n.Tr("settings.json says fullscreen; "+f.ForcedBy+" overrides it", "settings.json 写的是全屏，但被 "+f.ForcedBy+" 覆盖")
		}
		return dcheck{stOK, label, value, note}
	case f.knownSetting() == "fullscreen":
		return dcheck{stRec, label, i18n.Tr("fullscreen", "全屏"),
			why + i18n.Tr(". `gtmux doctor --fix` sets \"tui\": \"default\"", "。`gtmux doctor --fix` 会设为 \"tui\": \"default\"")}
	case f.knownSetting() == "default":
		if f.AltPanes > 0 {
			return stillFull(i18n.Tr("classic", "经典"))
		}
		return dcheck{stOK, label, i18n.Tr("classic", "经典"), i18n.Tr("tmux keeps scrollback", "tmux 保留滚动历史")}
	case f.AltPanes > 0:
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("fullscreen (%d of %d sessions)", "全屏（%d/%d 个会话）"), f.AltPanes, f.Panes),
			why + i18n.Tr(". Not pinned, so Claude chose it (a fresh install does); `gtmux doctor --fix` pins \"tui\": \"default\"",
				"。没有固定时由 Claude 自己选（新装默认全屏）；`gtmux doctor --fix` 会固定为 \"tui\": \"default\"") + unknown}
	case f.Panes == 0:
		// Nothing to look at: unset is exactly the state a fresh install turns fullscreen
		// in, so do not call it classic.
		return dcheck{stInfo, label, i18n.Tr("not pinned", "未固定"),
			i18n.Tr("no Claude session to check; a fresh install or update may start them fullscreen, and this row will say so",
				"当前没有可检查的 Claude 会话；新装或升级后可能以全屏启动，届时这一行会提示") + unknown}
	default:
		return dcheck{stOK, label, i18n.Tr("classic (not pinned)", "经典（未固定）"),
			i18n.Tr("a fresh install or update may switch to fullscreen; this row will say so",
				"新装或升级可能切到全屏，届时这一行会提示") + unknown}
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
		"  Pin Claude Code's classic renderer (\"tui\": \"default\" in "+tildeify(claudeSettingsPath())+").\n  Fullscreen runs in the alternate screen, where tmux keeps no scrollback. Backed up first to\n  "+tildeify(claudeTUIBackupPath())+"; nothing else in the file changes.",
		"  把 Claude Code 固定为经典渲染器（在 "+tildeify(claudeSettingsPath())+" 写入 \"tui\": \"default\"）。\n  全屏渲染器跑在备用屏幕里，tmux 不留滚动历史。会先备份到 "+tildeify(claudeTUIBackupPath())+"，文件其他内容不变。")
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

// claudeTUIBackupPath is this step's own backup. Not the hook step's rolling .gtmux.bak:
// in one --fix run both steps write settings.json, and a second backup to the same name
// would replace the file as it was before the run with the hook step's output.
func claudeTUIBackupPath() string { return claudeSettingsPath() + ".gtmux-tui.bak" }

// pinClaudeTUIDefault sets "tui": "default" and leaves the rest of the file's values as
// they were: numbers keep their digits and "<>&" stays literal (the shared helpers lose
// both). Key order is not kept — Go's JSON objects are unordered. A backup that cannot be
// written stops the change.
func pinClaudeTUIDefault(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	m := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			return err
		}
		if err := state.WriteForeign(claudeTUIBackupPath(), raw, 0o600); err != nil {
			return fmt.Errorf("backup %s: %w", tildeify(claudeTUIBackupPath()), err)
		}
	}
	m["tui"] = "default"
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return state.WriteForeign(path, out.Bytes(), 0o644)
}
