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
// file is edited. Removal deletes exactly the bytes from the opening sentinel through
// the closing one; nothing outside them is parsed or rewritten, so an install followed by
// an uninstall gives back the file byte for byte, trailing newlines included. The one
// byte gtmux may have to add outside the block (a line break, when the file did not end
// with one) is recorded inside the block and taken back with it. A block whose bounds
// are unclear (no closing sentinel, a closing one with no opening, or one opened inside
// another) is refused rather than guessed at: guessing "to the end of the file" deleted
// the user's own tables written after it (%12, 2026-10-06). Appending is always valid
// TOML: a table header ends the previous table's scope, so a `[[hooks]]` block at the
// end of a file cannot land inside someone else's table.
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
	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	kimiBlockBegin = "# >>> gtmux hooks — managed by `gtmux install hooks --agent kimi`"
	kimiBlockEnd   = "# <<< gtmux hooks"
	kimiBlockNote  = "# Remove this block (or run `gtmux uninstall hooks --agent kimi`) to unregister."
	// kimiBlockExact marks a block written by a gtmux that adds nothing outside it.
	// A block without it was written by an older one, which also put one blank line
	// before it (and trimmed the file's trailing newlines, which cannot be undone).
	kimiBlockExact = "# gtmux wrote only what lies between these two markers."
	// kimiBlockJoined replaces kimiBlockExact when the file did not end with a line
	// break and gtmux added one before the block; uninstall takes it back.
	kimiBlockJoined = "# gtmux also added the line break before this block: the file did not end with one."
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

// kimiBlock renders the managed block for this binary. joined says gtmux added the line
// break before it, so uninstall knows to take that byte back too.
func kimiBlock(bin string, joined bool) string {
	var b strings.Builder
	b.WriteString(kimiBlockBegin + "\n")
	b.WriteString(kimiBlockNote + "\n")
	if joined {
		b.WriteString(kimiBlockJoined + "\n")
	} else {
		b.WriteString(kimiBlockExact + "\n")
	}
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

// errKimiBlockUnclear is returned, before anything is written, when the sentinels do not
// pair up. The user's file and any earlier backup are left as they are.
type errKimiBlockUnclear struct{ line int }

func (e errKimiBlockUnclear) Error() string {
	return i18n.Tr(fmt.Sprintf("the gtmux block in this file has unclear bounds (line %d): each %q needs one %q after it. Nothing was changed. Fix or remove the block by hand, then run this again", e.line, kimiBlockBegin, kimiBlockEnd),
		fmt.Sprintf("这个文件里 gtmux 那块的边界不清（第 %d 行）：每个 %q 后面都要有一个 %q。什么都没改。请手动修好或删掉这块，再运行一次", e.line, kimiBlockBegin, kimiBlockEnd))
}

// stripKimiBlock removes every managed block from the text, byte for byte, returning
// the remainder and whether anything was found. It matches the sentinels alone and
// never parses TOML. Each block goes from the first byte of its opening line through
// the line break that ends its closing line; a block marked kimiBlockJoined also takes
// the line break before it, and an older block (no kimiBlockExact) the one blank line
// its gtmux put before it. Sentinels that do not pair up are an error, with nothing
// removed: what lies after an unclosed opening may be the user's own config.
func stripKimiBlock(text string) (string, bool, error) {
	type span struct {
		from, to int // byte offsets: [from, to)
		joined   bool
		exact    bool
	}
	var spans []span
	open := -1
	var cur span
	for off, n := 0, 1; off < len(text); n++ {
		end := strings.IndexByte(text[off:], '\n')
		next := len(text)
		if end >= 0 {
			next = off + end + 1
		}
		switch strings.TrimSpace(text[off:next]) {
		case kimiBlockBegin:
			if open >= 0 {
				return text, false, errKimiBlockUnclear{line: n}
			}
			open, cur = n, span{from: off}
		case kimiBlockEnd:
			if open < 0 {
				return text, false, errKimiBlockUnclear{line: n}
			}
			cur.to = next
			spans = append(spans, cur)
			open = -1
		case kimiBlockJoined:
			cur.joined = open >= 0
		case kimiBlockExact:
			cur.exact = cur.exact || open >= 0
		}
		off = next
	}
	if open >= 0 {
		return text, false, errKimiBlockUnclear{line: open}
	}
	if len(spans) == 0 {
		return text, false, nil
	}
	var b strings.Builder
	at := 0
	for _, sp := range spans {
		from := sp.from
		switch {
		case sp.joined:
			if from > at && text[from-1] == '\n' {
				from--
			}
		case !sp.exact:
			// An older gtmux wrote "\n\n" before its block: drop the blank line it added.
			if from-1 > at && text[from-1] == '\n' && text[from-2] == '\n' {
				from--
			}
		}
		b.WriteString(text[at:from])
		at = sp.to
	}
	b.WriteString(text[at:])
	return b.String(), true, nil
}

// updateKimiHooks writes or removes the managed block. Idempotent: an install always
// strips first, so running it twice leaves one block, and a gtmux that moved leaves no
// entry pointing at the old path. Nothing outside the block is changed, and a file whose
// block has unclear bounds is not written at all (nor backed up over an earlier backup).
func updateKimiHooks(path, bin string, install bool) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(raw)
	next, had, err := stripKimiBlock(text)
	if err != nil {
		return err
	}
	if !install {
		if !had {
			return nil // nothing of ours was there
		}
		backupFile(path)
		if next == "" {
			// The file existed only to hold our block — take it with us rather than
			// leaving an empty config behind.
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		return writeKimiConfig(path, next)
	}
	if len(raw) > 0 {
		backupFile(path)
	}
	joined := next != "" && !strings.HasSuffix(next, "\n")
	if joined {
		next += "\n"
	}
	return writeKimiConfig(path, next+kimiBlock(bin, joined))
}

func writeKimiConfig(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return state.WriteForeign(path, []byte(text), 0o644)
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
	i18n.Say("• written as one marked block at the end of the file; everything else in your config is untouched.",
		"• 以一整块带标记的内容追加在文件末尾，配置里的其他内容原样不动。")
	if installedAppPath() == "" {
		i18n.Say("• install the menu-bar app to get desktop notifications (curl installer, or 'make app')",
			"• 安装菜单栏 app 才能收到桌面通知（用 curl 安装脚本，或 'make app'）")
	}
	i18n.Say("Done. Kimi Code loads the hooks when it next starts, so restart it where it is already running.",
		"完成。Kimi Code 下次启动时才会加载 hook，已经在跑的那些重启一下。")
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
