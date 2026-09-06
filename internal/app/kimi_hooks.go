package app

// Kimi Code's hook installer — the first one that writes TOML, and the first that
// writes into a file gtmux does not own.
//
// Every other agent either has a hooks file of its own (Codex's hooks.json, Cursor's
// hooks.json) or takes a dedicated file gtmux writes whole (Kiro, Copilot, opencode's
// plugin). Kimi has neither: its hooks are `[[hooks]]` array-of-tables entries inside
// ~/.kimi-code/config.toml, the same file holding the user's providers, models and
// preferences. Rewriting that file as a parsed document would mean round-tripping
// someone's hand-written TOML — comments, ordering, formatting and all — through a
// serialiser, on every install, to change four lines at the bottom. gtmux does not
// have a TOML library and should not acquire one to do that.
//
// So this appends a MANAGED BLOCK, delimited by sentinel comments, the way a shell rc
// file is edited. Removal deletes exactly what lies between the sentinels; nothing
// else in the file is read, parsed, or rewritten. Appending is always valid TOML: a
// table header ends the previous table's scope, so a `[[hooks]]` block at the end of a
// file cannot land inside someone else's table.
//
// One constraint from Kimi's side shapes the block: `[[hooks]]` accepts exactly four
// fields (event, matcher, command, timeout) and an unknown field makes the WHOLE config
// fail to load — which would take the user's providers down with it. So there is no
// ownership marker inside the entry; the sentinel comments carry it.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

const (
	kimiBlockBegin = "# >>> gtmux hooks — managed by `gtmux install hooks --agent kimi`"
	kimiBlockEnd   = "# <<< gtmux hooks"
	// kimiHookTimeoutSec bounds one hook (Kimi's range is 1–600s, default 30). The
	// lifecycle hooks record and return; this is a backstop for a wedged invocation,
	// not a budget. Kimi is fail-open, so an expired hook costs a lost event, never
	// the user's turn.
	kimiHookTimeoutSec = 10
	// kimiFeedTimeoutSec covers the tool-feed events, which can sit behind a human
	// answering an approval prompt.
	kimiFeedTimeoutSec = 120
)

// kimiConfigPath is the file the block is appended to. KIMI_CODE_HOME relocates the
// whole data root, config included, so it is honoured here too.
func kimiConfigPath() string {
	if v := os.Getenv("KIMI_CODE_HOME"); v != "" {
		return filepath.Join(v, "config.toml")
	}
	return filepath.Join(homeDir(), ".kimi-code", "config.toml")
}

// kimiBinding is one native Kimi event mapped to a gtmux token.
type kimiBinding struct {
	event      string // Kimi's event name
	token      string // the token passed to `gtmux hook --agent kimi <token>`
	timeoutSec int
}

// kimiBindings is the event map. Kimi's payload is already snake_case and
// Claude-shaped (`hook_event_name` / `session_id` / `cwd` / `prompt`), so the hook
// needed no new parsing to read it.
var kimiBindings = []kimiBinding{
	{event: "UserPromptSubmit", token: "UserPromptSubmit", timeoutSec: kimiHookTimeoutSec},
	// Kimi raises a DEDICATED approval event, so PreToolUse stays telemetry rather
	// than being read as "needs you" — every tool call would otherwise flag one.
	{event: "PermissionRequest", token: "PermissionRequest", timeoutSec: kimiFeedTimeoutSec},
	{event: "PermissionResult", token: "PermissionResult", timeoutSec: kimiFeedTimeoutSec},
	{event: "Stop", token: "Stop", timeoutSec: kimiHookTimeoutSec},
	// A turn that died on an error is not a turn that finished. Without this the pane
	// would sit "working" until the next prompt.
	{event: "StopFailure", token: "StopFailure", timeoutSec: kimiHookTimeoutSec},
	{event: "SessionStart", token: "SessionStart", timeoutSec: kimiHookTimeoutSec},
	{event: "SessionEnd", token: "SessionEnd", timeoutSec: kimiHookTimeoutSec},
	{event: "PreToolUse", token: "PreToolUse", timeoutSec: kimiFeedTimeoutSec},
	{event: "PostToolUse", token: "PostToolUse", timeoutSec: kimiFeedTimeoutSec},
	{event: "PreCompact", token: "PreCompact", timeoutSec: kimiHookTimeoutSec},
	{event: "PostCompact", token: "PostCompact", timeoutSec: kimiHookTimeoutSec},
}

// kimiBlock renders the managed block for this binary.
func kimiBlock(bin string) string {
	var b strings.Builder
	b.WriteString(kimiBlockBegin + "\n")
	b.WriteString("# Remove this block (or run `gtmux uninstall hooks --agent kimi`) to unregister.\n")
	for _, k := range kimiBindings {
		b.WriteString("\n[[hooks]]\n")
		b.WriteString(fmt.Sprintf("event = %s\n", tomlString(k.event)))
		b.WriteString(fmt.Sprintf("command = %s\n", tomlString(fmt.Sprintf("%s hook --agent kimi %s", bin, k.token))))
		b.WriteString(fmt.Sprintf("timeout = %d\n", k.timeoutSec))
	}
	b.WriteString(kimiBlockEnd + "\n")
	return b.String()
}

// tomlString quotes a value as a TOML basic string. The only characters that can
// realistically appear here are in a filesystem path, but a path may contain a
// backslash or a quote and an unescaped one would corrupt the user's whole config.
func tomlString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`)
	return `"` + r.Replace(s) + `"`
}

// stripKimiBlock removes every managed block from the text, returning the remainder
// and whether anything was found.
//
// It matches on the sentinels alone and never parses TOML, so it cannot damage a
// neighbouring table. An unterminated block (someone deleted the closing sentinel)
// runs to the end of the file — which is where gtmux always puts it, so that is the
// truthful reading rather than a reason to give up and leave a duplicate behind.
func stripKimiBlock(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	found, inBlock := false, false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if !inBlock && t == kimiBlockBegin {
			inBlock, found = true, true
			continue
		}
		if inBlock {
			if t == kimiBlockEnd {
				inBlock = false
			}
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n"), found
}

// updateKimiHooks writes or removes the managed block. Idempotent: an install always
// strips first, so running it twice leaves one block, and a gtmux that moved leaves no
// entry pointing at the old path.
func updateKimiHooks(path, bin string, install bool) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(raw)
	if len(raw) > 0 {
		backupFile(path)
	}
	next, had := stripKimiBlock(text)
	if !install {
		if !had {
			return nil // nothing of ours was there
		}
		next = strings.TrimRight(next, "\n")
		if strings.TrimSpace(next) == "" {
			// The file existed only to hold our block — take it with us rather than
			// leaving an empty config behind.
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		return writeKimiConfig(path, next+"\n")
	}
	next = strings.TrimRight(next, "\n")
	if next != "" {
		next += "\n\n"
	}
	return writeKimiConfig(path, next+kimiBlock(bin))
}

func writeKimiConfig(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// installKimiHooks is the `install hooks --agent kimi` entry point.
func installKimiHooks(install bool) int {
	path := kimiConfigPath()
	if err := updateKimiHooks(path, selfPath(), install); err != nil {
		i18n.Sae(fmt.Sprintf("failed to update %s: %v", tildeify(path), err),
			fmt.Sprintf("更新 %s 失败：%v", tildeify(path), err))
		return 1
	}
	if !install {
		i18n.Say(fmt.Sprintf("✓ de-registered 'gtmux hook' for Kimi Code from %s", tildeify(path)),
			fmt.Sprintf("✓ 已为 Kimi Code 从 %s 注销 'gtmux hook'", tildeify(path)))
		return 0
	}
	i18n.Say(fmt.Sprintf("✓ registered 'gtmux hook' for Kimi Code in %s (UserPromptSubmit · PermissionRequest · Stop · Session start/end)", tildeify(path)),
		fmt.Sprintf("✓ 已为 Kimi Code 在 %s 注册 'gtmux hook'（UserPromptSubmit · PermissionRequest · Stop · Session 开始/结束）", tildeify(path)))
	i18n.Say("• written as one marked block at the end of the file — everything else in your config is untouched.",
		"• 以一整块带标记的内容追加在文件末尾 —— 配置里的其他内容原样不动。")
	if _, err := os.Stat(gtmuxAppPath()); err != nil {
		i18n.Say("• install the menu-bar app to get desktop notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能收到桌面通知（用 curl 安装脚本，或 'make app'）")
	}
	i18n.Say("Done. Restart Kimi Code sessions to load the hooks.",
		"完成。重启 Kimi Code 会话以加载 hook。")
	return 0
}

// kimiDataRoot is Kimi's data directory — its existence is how doctor decides whether
// to show a Kimi row at all, so a user who does not run Kimi never sees one.
func kimiDataRoot() string {
	if v := os.Getenv("KIMI_CODE_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir(), ".kimi-code")
}

// missingKimiHookEvents reports which bindings are absent from the installed block:
// nil when gtmux is not installed at all (a different fact, reported in its own
// words), an empty slice when everything is present.
//
// It reads the COMMANDS rather than the block's presence, because "the block is
// there" and "the block carries the events gtmux now uses" are different claims, and
// only the second one means the agent is fully wired. Nothing rewrites this file on
// update; that is `install hooks`, which nobody re-runs unprompted.
func missingKimiHookEvents() []string {
	raw, err := os.ReadFile(kimiConfigPath())
	if err != nil {
		return nil
	}
	text := string(raw)
	if !strings.Contains(text, "hook --agent kimi ") {
		return nil
	}
	missing := []string{}
	for _, b := range kimiBindings {
		// The trailing quote pins the whole token: without it "…kimi Stop" is also
		// matched by "…kimi StopFailure", and a missing Stop would read as present.
		if !strings.Contains(text, "hook --agent kimi "+b.token+`"`) {
			missing = append(missing, b.event)
		}
	}
	sort.Strings(missing)
	return missing
}
