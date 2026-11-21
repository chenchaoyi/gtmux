// `gtmux hq` — the supervisor (中控) session. Spawns (or focuses, when live) a
// dedicated tmux session running the user's coding agent in the persistent hq
// home (state.HQHome()), whose seeded playbook (AGENTS.md, with CLAUDE.md as an
// @-import for Claude) teaches the supervisor loop:
// read `gtmux digest --json` → judge → drill into a pane (tmux capture-pane)
// only when warranted → drive via `gtmux send` → report. The home keeps versioned
// managed rules and independent user records.
//
// The supervisor is deliberately "just an agent": it appears in the radar
// (marked role:"supervisor" via its cwd), jump/notifications work, and the
// phone can converse with it — no new machinery.
package hq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/hook"
	"github.com/chenchaoyi/gtmux/internal/hostplatform"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"golang.org/x/term"
)

// Bump the version when the shipped entry or any runbook changes. History is
// available in git; the fingerprint test covers both languages and every file.
const hqPlaybookVersion = 68

// playbookFingerprints files the charter text under the version that carries it, so an
// edit that forgets to bump the number fails instead of shipping to nobody (see
// playbook_version_test.go for why the floors above cannot catch that).
var playbookFingerprints = map[int]string{
	68: "b2f95bcc1ea2ac4b",
	67: "2cb3afde152fce06",
	66: "035a6dd5a27e114a",
	65: "a31739bfc2e0d0ba",
	64: "a15fe47202f4e190",
	63: "42d58cd9950f40cc",
	62: "e70d57ca0a35a34e",
	61: "c40ff19d752452f0",
	60: "2ac9abbfc3b8fab0",
	59: "9322033b5d9a0c3e",
	57: "60b9b7a7ed9a13d6",
	58: "15b5ac35f57fcaa3",
	56: "7a70c00da823fb4b",
	55: "bccecdff01b51072",
	36: "e2154b7242720d6b",
	37: "c8141b83a879aeb6",
	38: "bbcb7cd810ce0393",
	39: "51ebe25128e38c49",
	40: "0e5aa9f96309949d",
	41: "cbcc272429a4ba39",
	42: "7c373f1ec0d02ee6",
	43: "d7d477194088a85e",
	44: "8e1f13bf3f36bdd1",
	45: "38a4e9856e09bcf5",
	46: "5cc4177198ff4d27",
	47: "eb8fb873eac827e8",
	48: "51bebf41c0a260db",
	49: "63d8b7f6b840afa1",
	50: "fa8f7ecda47c5f0b",
	51: "79937a36a4ff3c89",
	52: "8c697f40dfe1271c",
	53: "0694c6ef968ce57b",
	54: "aed488f3e73ce1b3",
}

// playbookMarker is the machine-parseable managed-marker line prepended to the
// generated AGENTS.md: it stamps the version AND the charter language, and signals
// the file is gtmux-owned.
func playbookMarker(v int, lang string) string {
	return fmt.Sprintf("<!-- gtmux-hq-playbook v%d lang:%s · managed by gtmux — DO NOT EDIT; put your own instructions in LOCAL.md -->", v, lang)
}

var playbookVersionRe = regexp.MustCompile(`gtmux-hq-playbook v(\d+)`)

var playbookLangRe = regexp.MustCompile(`gtmux-hq-playbook v\d+ lang:(\w+)`)

// parsePlaybookLang reads the charter language from an AGENTS.md body's marker.
// A marker without lang: (written before bilingual-playbook) parses as "", which
// never equals a real language — so a legacy install regenerates exactly once.
func parsePlaybookLang(body string) string {
	if m := playbookLangRe.FindStringSubmatch(body); len(m) == 2 {
		return m[1]
	}
	return ""
}

// parsePlaybookVersion reads the version from the managed marker. A missing
// marker parses as 0 so startup and refresh can recognize unmanaged text.
func parsePlaybookVersion(body string) int {
	if m := playbookVersionRe.FindStringSubmatch(body); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// hqLocalPath is the user's personalization file — seed-once, NEVER overwritten.
func hqLocalPath() string { return filepath.Join(state.HQHome(), "LOCAL.md") }

// hqLocalTemplate is the seed-once LOCAL.md personalization template, in the
// user's language (GTMUX_LANG) — like the board seed and the charter. Seed-once
// means a later language switch never rewrites an existing LOCAL.md: the file is
// the user's from the first byte.
func hqLocalTemplate() string {
	if i18n.Lang() == "zh" {
		return `# 你的 HQ 指令(LOCAL.md)

gtmux 绝不覆盖这个文件。托管守则(AGENTS.md)会在 gtmux 发布新版时重新生成;
启动时载入或主动阅读此文件。你的规则优先于托管默认。详尽流程可用普通链接按需引用。

这个文件只约束中控。通知、推送、勿扰模式是 gtmux 自己的设置
(config.json / 各 app)——写在这里的规则静音不了手机。

<!-- 在下方添加你的常备指令、偏好与覆盖规则。示例:

- 我主要在做: <你的领域 / 主要仓库>。
- 状态汇报方式: <如:简洁表格;先说需要我的>。
- 免打扰时段: <如:22:00 后例行汇报先压着;阻塞项照样上报>。

-->
`
	}
	return `# Your HQ instructions (LOCAL.md)

gtmux NEVER overwrites this file. The managed playbook (AGENTS.md) is regenerated when
gtmux ships a newer version. HQ loads or reads this file at startup and applies your
rules before managed defaults. Use ordinary links to detailed procedures when useful.

This file steers the SUPERVISOR only. Notifications, push, and quiet-mode mechanics are
gtmux's own settings (config.json / the apps) — a rule here cannot mute the phone.

<!-- Add your own standing instructions, preferences, and overrides below. Examples:

- I mainly work on: <your domains / main repos>.
- Report status as: <e.g. terse tables; lead with what needs me>.
- Quiet hours: <e.g. after 22:00, hold routine reports; still surface blockers>.

-->
`
}

// hqSessionName is the preferred tmux session name (auto-named on collision —
// detection uses the HQ stamp/cwd, not this cosmetic name).
const hqSessionName = "Gtmux HQ"

// hqAgentCommand is what gets typed into the fresh hq pane when --agent is not
// given. GTMUX_HQ_AGENT overrides the default (e.g. "codex", or a command with
// env prefixes like the home-VPN proxy); the --agent flag beats both.
func hqAgentCommand() string {
	if c := strings.TrimSpace(os.Getenv("GTMUX_HQ_AGENT")); c != "" {
		return c
	}
	return "claude"
}

// hqInstructionsPath is the CANONICAL seeded playbook inside the hq home:
// AGENTS.md is read natively by Codex. Other clients may need an explicit read.
func hqInstructionsPath() string { return filepath.Join(state.HQHome(), "AGENTS.md") }

// hqClaudePointerPath is the Claude-side entry: Claude Code reads CLAUDE.md, so
// it gets a one-line `@AGENTS.md` import — SAME content, single source of truth,
// no two-file drift.
func hqClaudePointerPath() string { return filepath.Join(state.HQHome(), "CLAUDE.md") }

// hqClaudePointer is CLAUDE.md's content: Claude Code's @-import pulls the
// canonical AGENTS.md so both agent families read ONE playbook.
const hqClaudePointer = `@AGENTS.md
`

// seedHQHome installs a managed entry and runbooks without replacing the user's
// LOCAL or a full legacy CLAUDE.md. Unknown unversioned AGENTS.md is left active.
func seedHQHome(wantLang string) (seedResult, error) {
	var r seedResult
	home := state.HQHome()
	if err := state.MkdirAll(home, 0o755); err != nil {
		return r, err
	}
	_, entryErr := os.Lstat(hqInstructionsPath())
	switch {
	case errors.Is(entryErr, os.ErrNotExist):
		lang := wantLang
		if lang == "" {
			lang = i18n.Lang()
		}
		if _, err := ensureRunbooks(lang); err != nil {
			return r, err
		}
		if err := state.WritePrivateFile(hqInstructionsPath(), []byte(playbookIn(lang))); err != nil {
			return r, err
		}
		r.Seeded, r.ToVersion = true, hqPlaybookVersion
	case entryErr != nil:
		return r, entryErr
	default:
		if err := upgradePlaybookIfNewer(&r, wantLang); err != nil {
			return r, err
		}
	}
	if _, err := os.Lstat(hqClaudePointerPath()); errors.Is(err, os.ErrNotExist) {
		if err := state.WritePrivateFile(hqClaudePointerPath(), []byte(hqClaudePointer)); err != nil {
			return r, err
		}
	} else if err != nil {
		return r, err
	}
	if seedHQLocal() {
		r.Seeded = true
	}
	if seedHQKnowledge() {
		r.Seeded = true
	}
	if seedHQNotes() {
		r.Seeded = true
	}
	return r, nil
}

// writeHQBackup creates an exclusive, verified backup without retiring prior copies.
func writeHQBackup(path, stem string, body []byte) (string, error) {
	for i := 1; i <= 100; i++ {
		bak := stem
		if i > 1 {
			bak = fmt.Sprintf("%s-%d", stem, i)
		}
		f, err := state.OpenFile(bak, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		defer f.Close()
		created, err := f.Stat()
		if err != nil {
			return "", err
		}
		_, werr := f.Write(body)
		serr := f.Sync()
		if err := errors.Join(werr, serr); err != nil {
			return "", err
		}
		if hqBackupWritten != nil {
			hqBackupWritten(bak)
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		got, err := io.ReadAll(io.LimitReader(f, int64(len(body))+1))
		if err != nil || !bytes.Equal(got, body) {
			return "", fmt.Errorf("the backup %s did not read back", bak)
		}
		current, err := os.Lstat(bak)
		if err != nil || !current.Mode().IsRegular() || !os.SameFile(created, current) {
			return "", fmt.Errorf("the backup %s changed during verification; retained without cleanup", bak)
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return bak, nil
	}
	return "", fmt.Errorf("no free backup name next to %s", path)
}

// seedResult reports what seedHQHome did, so `gtmux hq` can print the right notice:
// a fresh seed, a version UPGRADE (with the prior backed up), or a legacy MIGRATION.
type seedResult struct {
	Seeded           bool   // created at least one file in a fresh/incomplete home
	Upgraded         bool   // regenerated a managed AGENTS.md at a newer version
	Preserved        bool   // existing entry may carry user policy and requires review
	RunbooksRepaired bool   // an installed edition was missing a linked runbook
	FromVersion      int    // the installed version before an upgrade
	ToVersion        int    // the shipped version written
	LangSwitch       string // non-empty: the user asked for a language change (--lang), to this language
	// LangDiffers is set when the charter is in one language and this shell reads
	// another, and NOTHING was changed because of it. It exists so the mismatch can be
	// mentioned once instead of acted on.
	LangDiffers string
	BackupPath  string // where the prior AGENTS.md was backed up (on upgrade)
}

// Startup recognizes the older language-less managed marker as well. An
// unrecognized or mixed entry stays active for the user's review.
func upgradePlaybookIfNewer(r *seedResult, wantLang string) error {
	f, err := state.OpenPrivateRead(hqInstructionsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0200 == 0 {
		r.Preserved = true
		return nil
	}
	const limit = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if len(body) > limit {
		r.Preserved = true
		return nil
	}
	version, lang := parsePlaybookVersion(string(body)), parsePlaybookLang(string(body))
	marker, _, _ := strings.Cut(string(body), "\n")
	expected := playbookMarker(version, lang)
	if lang == "" {
		expected = strings.Replace(expected, " lang:", "", 1)
	}
	if version == 0 || marker != expected || (lang != "" && lang != "en" && lang != "zh") || checkIndependentRules(body) != nil {
		r.Preserved = true
		return nil
	}
	return upgradePlaybookBody(r, wantLang, body, publishStartupPlaybook)
}

// The renderer and version/language decision are shared; rules-only maintenance
// uses a stricter reader and publisher without invoking startup's other actions.
func upgradePlaybookBody(r *seedResult, wantLang string, body []byte, publish func(string, []byte, []byte) (string, error)) error {
	installed := parsePlaybookVersion(string(body))
	installedLang := parsePlaybookLang(string(body))

	// Which language to WRITE, if we write at all. The default is the language the
	// charter is already in — never the shell's.
	//
	// It used to be the shell's, so a terminal with a different GTMUX_LANG rewrote the
	// user's charter into another language as a side effect of running `gtmux hq` for
	// some unrelated reason. Nobody asked for that, and a translated charter is not a
	// small change: it is the document the supervisor works from. Changing it is now
	// something the user asks for by name (`gtmux hq --lang zh`), and everything else
	// leaves it alone.
	writeLang := installedLang
	if writeLang == "" {
		writeLang = i18n.Lang() // legacy file with no marker: nothing to preserve
	}
	if wantLang != "" {
		writeLang = wantLang
	}

	if installed > hqPlaybookVersion || (installed == hqPlaybookVersion && writeLang == installedLang) {
		if installed == hqPlaybookVersion && string(body) == playbookIn(installedLang) {
			var err error
			r.RunbooksRepaired, err = ensureRunbooks(installedLang)
			if err != nil {
				return err
			}
		}
		// Nothing to write. If this shell reads a different language than the charter
		// is in, say so once — an advisory, not an action.
		if installedLang != "" && installedLang != i18n.Lang() {
			if !r.RunbooksRepaired {
				r.LangDiffers = installedLang
			}
		}
		return nil
	}
	// Back up the prior playbook before overwriting, keyed by its version+language so
	// no upgrade ever clobbers an earlier backup.
	bakLang := installedLang
	if bakLang == "" {
		bakLang = "legacy"
	}
	bak := hqInstructionsPath() + fmt.Sprintf(".bak-v%d-%s", installed, bakLang)
	bak, err := publish(bak, body, []byte(playbookIn(writeLang)))
	if err != nil {
		return err
	}
	r.Upgraded = true
	r.FromVersion, r.ToVersion = installed, hqPlaybookVersion
	if installedLang != "" && writeLang != installedLang {
		r.LangSwitch = writeLang // only ever set by an explicit --lang
	}
	r.BackupPath = bak
	return nil
}

func publishStartupPlaybook(stem string, prior, replacement []byte) (string, error) {
	backup, err := writeHQBackup(hqInstructionsPath(), stem, prior)
	if err != nil {
		return "", err
	}
	if _, err := ensureRunbooks(parsePlaybookLang(string(replacement))); err != nil {
		return backup, err
	}
	return backup, state.WritePrivateFile(hqInstructionsPath(), replacement)
}

// Seed the user file exclusively, including when an existing path is dangling.
func seedHQLocal() bool {
	f, err := state.OpenFile(hqLocalPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return false
	}
	_, writeErr := io.WriteString(f, hqLocalTemplate())
	closeErr := f.Close()
	return writeErr == nil && closeErr == nil
}

// printSeedNotice describes changes or an existing entry that needs review.
func printSeedNotice(r seedResult) {
	switch {
	case r.Preserved:
		i18n.Sae("Kept the existing HQ entry active. Review unmanaged or independent instructions before converting it to managed rules.",
			"已保留现有 HQ 入口。转用托管规则前，请先核对旧式或独立指令。")
	case r.RunbooksRepaired:
		i18n.Say("Restored missing managed HQ runbooks.", "已恢复缺失的 HQ 托管手册。")
	case r.Upgraded && r.LangSwitch != "" && r.FromVersion == r.ToVersion:
		i18n.Say(fmt.Sprintf("Rewrote the HQ charter in %s, as you asked (the old one is saved at %s). Your %s is untouched.",
			r.LangSwitch, r.BackupPath, filepath.Base(hqLocalPath())),
			fmt.Sprintf("已按你的要求把 HQ 守则改写成 %s（旧的存在 %s）。你的 %s 没动。",
				r.LangSwitch, r.BackupPath, filepath.Base(hqLocalPath())))
	case r.Upgraded:
		i18n.Say(fmt.Sprintf("Upgraded the HQ playbook v%d → v%d (previous backed up at %s). Your %s is untouched.",
			r.FromVersion, r.ToVersion, r.BackupPath, filepath.Base(hqLocalPath())),
			fmt.Sprintf("已升级 HQ 守则 v%d → v%d（旧版备份在 %s）。你的 %s 未改动。",
				r.FromVersion, r.ToVersion, r.BackupPath, filepath.Base(hqLocalPath())))
	case r.LangDiffers != "":
		// Nothing happened here, and that is the point: the charter is in one language,
		// this terminal reads another, and gtmux left the file alone. Say all four
		// things a person needs — what the charter is, what this shell wants, that
		// nothing changed, and the one command that would change it.
		other := "zh"
		if r.LangDiffers == "zh" {
			other = "en"
		}
		i18n.Say(fmt.Sprintf("The HQ charter is written in %s; this terminal is set to %s. Nothing was changed. To rewrite it in %s:  gtmux hq --lang %s",
			r.LangDiffers, i18n.Lang(), other, other),
			fmt.Sprintf("HQ 守则是 %s 的，你这个终端用的是 %s。什么都没改。想改成 %s：  gtmux hq --lang %s",
				r.LangDiffers, i18n.Lang(), other, other))
	case r.Seeded:
		i18n.Say("Seeded the HQ home: "+hqInstructionsPath()+
			"\n  · knowledge/ is its knowledge base (ledger + topics); anyone can file a candidate: gtmux capture \"<lesson> @pitfalls\""+
			"\n  · LOCAL.md holds your own standing instructions (never overwritten; edit this one, leave AGENTS.md to gtmux)",
			"已初始化 HQ 目录："+hqInstructionsPath()+
				"\n  · knowledge/ 是它的知识库（台账 + 主题），任何人都可以投候选：gtmux capture \"<教训> @pitfalls\""+
				"\n  · LOCAL.md 放你自己的常设指令（永不被覆盖；要改就改它，AGENTS.md 交给 gtmux）")
	}
}

// hqNotesDir is HQ's private working area (its situation board + any scratch notes).
func hqNotesDir() string { return filepath.Join(state.HQHome(), "notes") }

// seedHQNotes lays down the situation-board template IF ABSENT —HQ's durable command
// posture that survives a context reset. Written only when missing, so HQ's curated
// board is never overwritten. Returns whether it created anything.
func seedHQNotes() (created bool) {
	dir := hqNotesDir()
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	for name, body := range hqNotesSeeds() {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			if state.WriteFile(p, []byte(body), 0o644) == nil {
				created = true
			}
		}
	}
	return created
}

// hqNotesSeeds is the notes scaffold — the situation board HQ maintains as its
// cross-turn posture (curated markdown, NOT a gtmux-parsed schema).
func hqNotesSeeds() map[string]string {
	return map[string]string{"board.md": boardSeed()}
}

// boardSeedHeadings are the board's two section headings and the ① table header,
// in the language the home is being seeded in (hq-first-person): the board is the
// USER's readable surface, so its skeleton follows GTMUX_LANG. The playbook's
// rule is then "keep the board in the language it was seeded in".
// AskHeadingEN / AskHeadingZH are the ONE heading gtmux owns inside the board.
//
// Everything else on the board is HQ's to name. This one is not, because the surfaces
// hoist it: it is the only part of the board that is the COMMANDER's business, and it was
// a third-level heading inside ①, reachable only by expanding the section above it
// (2026-09-09). A surface can only lift a section it can recognise, so the name is
// gtmux's and both spellings are matched — a board seeded in English keeps its English
// heading, and a board that has neither simply shows no band.
const (
	AskHeadingEN = "Still waiting on you"
	AskHeadingZH = "还等你定的"
)

// IsAskHeading reports whether a board heading is the one the surfaces hoist.
func IsAskHeading(title string) bool {
	t := strings.TrimSpace(strings.TrimLeft(title, "# "))
	return t == AskHeadingEN || t == AskHeadingZH
}

func boardSeedHeadings() (h1, cols, h2 string) {
	return i18n.Tr("## ① Now — live panes", "## ① 现状 — 在跑的 pane"),
		i18n.Tr("| pane | doing | dispatched by | priority | state | your call | lesson |",
			"| pane | 在做什么 | 谁派的 | 优先级 | 状态 | 等你定 | 教训 |"),
		i18n.Tr("## ② Handoff log — newest first", "## ② 交接记录 — 新的在最上面")
}

func boardSeed() string {
	h1, cols, h2 := boardSeedHeadings()
	oneIntro := i18n.Tr(
		"One row per LIVE pane, named by its `%N`. Delete a row when that pane's work is finished —\nthis part has no history in it, only now.",
		"一行一个在跑的 pane,以其 `%N` 命名;活干完就删行 —— 这一部分只有现在,没有历史。")
	example := i18n.Tr(
		"| `%23` | _what it is doing_ | HQ / you directly / self-started | high/med/low | ok / stuck / erroring | _what it awaits from you_ | _last correction or footgun_ |",
		"| `%23` | _它在做什么_ | 中控派的 / 你直接说的 / 它自己起的 | 高/中/低 | 正常 / 卡住 / 出错 | _在等你定什么_ | _上一次的纠正或坑_ |")
	ask := i18n.Tr("### "+AskHeadingEN, "### "+AskHeadingZH)
	askIntro := i18n.Tr(
		"What the commander has to decide, one line each, named by the pane that is blocked.\nEmpty is the normal state — delete a line the moment it is answered. Both the phone and\nthe menu bar LIFT this section to the top of the board, so it is the first thing they see:\nput a question here only when you genuinely cannot decide it yourself.",
		"需要司令拍板的事,一件一行,用被挡住的那个 pane 命名。\n空着是常态 —— 一旦有了答复就立刻删掉那一行。手机和菜单栏都会把这一节**提到板的最上面**,\n那是他们第一眼看到的东西:只有你确实无法自行决定时才往这里写。")
	return boardSeedTop + h1 + "\n\n" + oneIntro + "\n\n" +
		cols + "\n|---|---|---|---|---|---|---|\n" + example + "\n\n" +
		ask + "\n\n" + askIntro + "\n\n" +
		h2 + "\n" + boardSeedTail
}

const boardSeedTop = `# gtmux HQ— situation board (作战态势板)

Your DURABLE command posture. gtmux does NOT read this back — it is your synthesis,
kept current by you, so your picture of the fleet survives a context compaction or
reset (Claude Code's ` + "`/compact`" + ` — your agent's equivalent applies). After a reset, RE-READ this before acting instead of re-deriving the fleet from
scratch. The deterministic truth is ` + "`gtmux digest` / `gtmux tasks` / `gtmux events`" + ` —
this board is where you record what they don't: mode, priority, pending decisions, lessons.

This file has TWO parts and they do not mix. Keep both; keep both short.

`

// boardSeedTail is everything below the ② heading: dating discipline, the
// language rule (keep the seeded language; never machine-translate), emphasis
// discipline, and the standing-context section.
const boardSeedTail = `
One dated entry per rotation or notable shift, newest at the top. Never append a dated
entry to the end: a board that runs one way at the top and the other way at the bottom
takes a map to read, and the commander reads this on a phone.

KEEP THIS BOARD IN THE LANGUAGE IT WAS SEEDED IN — never flip an existing board, and
never translate a line word-for-word: say it the way you would say it out loud in that
language. (A literal translation once produced headings no native speaker would write;
the cure is writing naturally, not switching languages.)

Emphasis is for exceptions. If every line is bold, the headings stop being headings.

## Standing context (survives resets)

_The commander's current priorities, discussed directions in flight, and any mode-③
delegations already agreed — so you know what is "in an already-discussed direction"._
`

// isClaudePointer reports whether CLAUDE.md is just the `@AGENTS.md` import (the
// single-source pointer) rather than a full standalone playbook.
func isClaudePointer(body string) bool {
	return strings.TrimSpace(body) == strings.TrimSpace(hqClaudePointer)
}

// hqPolicyWarning returns a non-empty advisory (en, zh) when the home's policy layout
// is redundant or broken, so `gtmux hq` surfaces it instead of silently living with a
// zombie/dangling doc. "" when the layout is clean (single source, or a lone doc).
func hqPolicyWarning() (en, zh string) {
	hasAgents := fileExists(hqInstructionsPath())
	body, readErr := os.ReadFile(hqClaudePointerPath())
	hasClaude := readErr == nil
	switch {
	case hasAgents && hasClaude && !isClaudePointer(string(body)):
		// A full CLAUDE.md + AGENTS.md: Claude reads only CLAUDE.md, so AGENTS.md is a
		// redundant copy that drifts.
		return "hq home has TWO policy docs — a full CLAUDE.md and AGENTS.md. Claude loads the existing CLAUDE.md. Preserve its personal rules when reviewing a switch to the managed import.",
			"HQ home 有两份政策文档:全文 CLAUDE.md 和 AGENTS.md。Claude 会装载现有 CLAUDE.md。审阅是否切换托管引用时，请保留其中的个人规则。"
	case hasClaude && isClaudePointer(string(body)) && !hasAgents:
		// CLAUDE.md imports @AGENTS.md but AGENTS.md is gone → dangling import.
		return "hq CLAUDE.md imports `@AGENTS.md` but AGENTS.md is missing — Claude will load no playbook. Restore AGENTS.md, or put the playbook directly in CLAUDE.md.",
			"HQ CLAUDE.md 引用了 `@AGENTS.md` 但 AGENTS.md 不存在 —— Claude 读不到守则。恢复 AGENTS.md,或把守则直接写进 CLAUDE.md。"
	}
	return "", ""
}

// hqKnowledgeDir is the supervisor's living knowledge base (its primary long-term
// value — see the playbook). Topic files persist across sessions.
func hqKnowledgeDir() string { return filepath.Join(state.HQHome(), "knowledge") }

// seedHQKnowledge lays down the knowledge-base scaffold (README + empty topic
// files) IF ABSENT — each file only when missing, so the supervisor's curated
// content is never overwritten. Returns whether it created anything.
func seedHQKnowledge() (created bool) {
	dir := hqKnowledgeDir()
	if err := state.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	for name, body := range knowledge.TopicSeeds {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			if state.WriteFile(p, []byte(body), 0o644) == nil {
				created = true
			}
		}
	}
	return created
}

// hqAgentAlive reports whether a coding agent is actually the FOREGROUND process in the
// HQ pane. A stamped pane that has dropped back to an interactive shell means the
// supervisor exited (the user quit it) — so `gtmux hq` should relaunch it, not focus a
// dead prompt. Missing pane / no command reads as not-alive.
func hqAgentAlive(pane string) bool {
	// The SUBTREE first: is a known agent actually running under this pane?
	//
	// The foreground command alone cannot answer it. It is unreliable in both
	// directions — a live Claude pane reports its VERSION ("2.1.220"), not "claude", so
	// a name test would call a running supervisor dead; and anything that is not a shell
	// passes, so `gtmux hq` typed at a bare prompt in the HQ home made GTMUX ITSELF the
	// evidence that a supervisor was running. The operator got "already running, taking
	// you there" and no supervisor (report, 2026-09-07).
	//
	// A subtree walk answers the question that was actually being asked. It also gets
	// the case where HQ shells out `gtmux hq` from inside itself right: the agent is
	// still its own ancestor there, so it correctly reads as alive.
	if key := radar.AgentDriverKey(pane); key != "" {
		return true
	}
	return agentAliveByCmd(tmux.Display(pane, "#{pane_current_command}"))
}

// agentAliveByCmd is the FALLBACK decision, for a pane whose subtree named no known
// agent: a live agent's foreground command is anything that is not an interactive shell
// and is not gtmux itself.
//
// Kept because the agent registry cannot know every agent anyone runs, so "something
// that is not a shell is holding this pane" remains useful evidence. gtmux is excluded
// by name because gtmux running in a pane says only that someone typed a gtmux command
// there — which is precisely what the person asking this question just did.
func agentAliveByCmd(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || isShellCommand(cmd) {
		return false
	}
	return filepath.Base(strings.Fields(cmd)[0]) != "gtmux"
}

// CmdHQ implements `gtmux hq`: focus the live supervisor, or seed + spawn one.
func CmdHQ(args []string) int {
	// Inspect the standalone token before another option can consume it as a
	// value or choose a mode with startup/enrollment side effects.
	for _, arg := range args {
		if arg != "--refresh-rules" {
			continue
		}
		for _, help := range args {
			if help == "-h" || help == "--help" || help == "--help-all" {
				fmt.Print(HelpText(helpWidth(), help == "--help-all"))
				return 0
			}
		}
		if len(args) != 1 {
			i18n.Sae("gtmux hq: --refresh-rules must be used on its own", "gtmux hq：--refresh-rules 请单独使用")
			return 2
		}
		return refreshHQRulesCmd()
	}
	if len(args) > 0 && args[0] == "migrate" {
		return migrationCmd(args[1:])
	}
	agentCmd := ""
	rotate := false
	board := false       // --board: print the situation board instead of opening HQ
	boardJSON := false   // --json alongside --board, for a surface that wants the mtime too
	home := false        // --home: print where a knowledge mutation has to run
	memoryState := false // --memory: what is at risk and what protects it
	exportTo := ""       // --export <path>: the memory as one portable file
	expectedArchive := ""
	importFrom := ""        // --import <path>: put one back
	exportPlain := false    // --plain: an export without a passphrase (the pre-1.0.21 form)
	passStdin := false      // --passphrase-stdin: the passphrase is the first line of stdin
	charterLang := ""       // --lang: the ONLY way the charter's language ever changes
	var target launchTarget // --pane / --here / --new-pane: where to put the supervisor
	briefPane := ""         // --brief-pane: run as the detached briefing watcher
	maintenanceDone := ""   // HQ's completion receipt for a requested pass
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help" || a == "--help-all":
			fmt.Print(HelpText(helpWidth(), a == "--help-all"))
			return 0
		case a == "--rotate":
			rotate = true
		case a == "--maintenance-done":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --maintenance-done needs distill or self-check", "gtmux hq: --maintenance-done 需要 distill 或 self-check")
				return 2
			}
			i++
			maintenanceDone = args[i]
		case a == briefPaneFlag: // hidden: the detached half of a briefing into gtmux's own pane
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: "+briefPaneFlag+" needs a pane id", "gtmux hq: "+briefPaneFlag+" 需要一个 pane id")
				return 2
			}
			i++
			briefPane = args[i]
		case a == "--pane":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --pane needs a pane id like %21", "gtmux hq: --pane 需要一个 pane id，形如 %21")
				return 2
			}
			i++
			target.pane = args[i]
		case strings.HasPrefix(a, "--pane="):
			target.pane = strings.TrimPrefix(a, "--pane=")
		case a == "--here":
			target.here = true
		case a == "--new-pane":
			target.newPane = true
		case a == "--board":
			board = true
		case a == "--home":
			home = true
		case a == "--records", a == "--memory": // --memory: the pre-1.0.21 spelling, kept
			memoryState = true
		case a == "--export":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --export needs a path", "gtmux hq: --export 需要一个路径")
				return 2
			}
			i++
			exportTo = args[i]
		case strings.HasPrefix(a, "--export="):
			exportTo = strings.TrimPrefix(a, "--export=")
		case a == "--plain":
			exportPlain = true
		case a == "--passphrase-stdin":
			passStdin = true
		case a == "--expect-archive":
			if i+1 >= len(args) {
				i18n.Sae("--expect-archive needs a digest", "--expect-archive 需要内容摘要")
				return 2
			}
			i++
			expectedArchive = args[i]
		case a == "--import":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --import needs a path", "gtmux hq: --import 需要一个路径")
				return 2
			}
			i++
			importFrom = args[i]
		case strings.HasPrefix(a, "--import="):
			importFrom = strings.TrimPrefix(a, "--import=")
		case a == "--json":
			boardJSON = true
		case a == "--agent":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --agent needs a command", "gtmux hq: --agent 需要一个命令")
				return 2
			}
			i++
			agentCmd = args[i]
		case strings.HasPrefix(a, "--agent="):
			agentCmd = strings.TrimPrefix(a, "--agent=")
		case a == "--lang":
			if i+1 >= len(args) {
				i18n.Sae("gtmux hq: --lang needs a language (en or zh)", "gtmux hq: --lang 需要一个语言（en 或 zh）")
				return 2
			}
			i++
			charterLang = args[i]
		case strings.HasPrefix(a, "--lang="):
			charterLang = strings.TrimPrefix(a, "--lang=")
		default:
			i18n.Sae("gtmux hq: unknown option '"+a+"'", "gtmux hq: 未知选项 '"+a+"'")
			return 2
		}
	}
	// The detached briefing watcher (self-pane hand-over, see briefhere.go) does one thing
	// and exits; it never seeds, spawns or focuses anything.
	if briefPane != "" {
		return briefPaneWorker(briefPane, agentCmd)
	}
	if maintenanceDone != "" {
		if len(args) != 2 {
			i18n.Sae("gtmux hq: --maintenance-done cannot be combined with other options",
				"gtmux hq: --maintenance-done 不能与其它参数同时使用")
			return 2
		}
		if err := completeMaintenance(maintenanceDone, time.Now().Unix()); err != nil {
			diag.Did("act.hq.maintenance", maintenanceDone, diag.Refused,
				"HQ maintenance completion was refused", "reason", err.Error())
			i18n.Sae("gtmux hq: "+err.Error(), "gtmux hq: "+err.Error())
			return 1
		}
		return 0
	}
	// --board is a READ, and it takes the whole command: a surface that wants HQ's
	// synthesis must not have to know where the HQ home lives (it is relocatable, and on
	// this machine it is reached through a symlink). The menu-bar app is a CLI consumer by
	// construction, so the CLI is where that path resolution belongs.
	if expectedArchive != "" && importFrom == "" {
		i18n.Sae("--expect-archive requires --import", "--expect-archive 需与 --import 一起使用")
		return 2
	}
	if board {
		return printBoard(boardJSON)
	}
	if home {
		return printHQHome()
	}
	if memoryState {
		return printMemoryState(boardJSON)
	}
	if exportTo != "" {
		return diag.DidRC("act.hq.export", "records", exportMemoryCmd(exportTo, exportPlain, passStdin),
			"exported HQ's records to one file", "locked", !exportPlain)
	}
	if importFrom != "" {
		return diag.DidRC("act.hq.import", "records", importMemoryCmd(importFrom, passStdin, expectedArchive),
			"imported HQ's records from a file")
	}
	if charterLang != "" && charterLang != "en" && charterLang != "zh" {
		i18n.Sae("gtmux hq: --lang takes en or zh, not '"+charterLang+"'",
			"gtmux hq: --lang 只接受 en 或 zh，不是 '"+charterLang+"'")
		return 2
	}
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux（brew install tmux）")
		return 1
	}
	// --rotate acts on the LIVE supervisor and nothing else: no seeding, no spawn, no
	// focus. It is HQ's own hand on its own session (see the self-rotate sensor), so it
	// must never be the thing that CREATES a supervisor — an HQ that isn't running has no
	// session to retire, and saying so plainly beats silently starting one.
	if rotate {
		req, ok, held := RequestHQRotation()
		if !ok {
			diag.Did("act.hq.rotate", "hq", diag.Refused, "HQ rotation was not queued", "reason", held)
			i18n.Sae("gtmux hq --rotate: "+held, "gtmux hq --rotate: "+held)
			return 1
		}
		i18n.Say("HQ rotation queued for session "+req.Retiring+"; it will run after this turn ends. Codex completion requires a recoverable successor and confirmed handoff.",
			"已为 HQ 会话 "+req.Retiring+" 安排轮换；本回合结束后执行。Codex 需确认新会话已接棒且可恢复。")
		return 0
	}

	release, lockErr := acquireMigrationApplyLock()
	if lockErr != nil {
		i18n.Sae("gtmux hq: "+lockErr.Error(), "gtmux hq："+lockErr.Error())
		return 1
	}
	defer release()

	radar.PreflightResource() // warn (not block) if a machine resource is at its red line
	res, err := seedHQHome(charterLang)
	if err != nil {
		i18n.Sae("gtmux hq: "+err.Error(), "gtmux hq: "+err.Error())
		return 1
	}
	printSeedNotice(res)
	// Enrollment baseline (hq-perception-v2): mark every currently-live pane as
	// enrolled —HQ's seeded first turn does the FULL fleet enrollment, so only
	// panes appearing AFTER this point fire an incremental `new-session` wake.
	hook.StampEnrolledAll()
	// ④ Surface a redundant/broken policy layout instead of silently living with it.
	if en, zh := hqPolicyWarning(); en != "" {
		i18n.Sae("gtmux hq: "+en, "gtmux hq: "+zh)
	}

	// An explicit place for the supervisor overrides the search below: the user has
	// answered the question it exists to answer.
	if target.any() {
		kind, at, err := resolveLaunchTarget(target, ownPane())
		if err != nil {
			i18n.Sae("gtmux hq: "+err.Error(), "gtmux hq: "+err.Error())
			return 2
		}
		return launchHQAt(kind, at, agentCmd)
	}

	// A stamped HQ pane exists — but is the supervisor AGENT actually alive in it? If
	// the user quit the agent and left the tmux window (a bare shell), the stamp
	// lingers; focusing it would land on a dead prompt and claim "Focused the running
	// supervisor" — confusing. So check the pane's foreground command: a shell means
	// the agent exited → relaunch it IN PLACE (reuse the window the user already has),
	// never a dead-window focus.
	if pane := hqpane.Find(); pane != "" {
		where := hqWhere(pane)
		if hqAgentAlive(pane) {
			nameHQWindow(pane, false)
			// Say it BEFORE moving. This branch used to print after the jump, so the line
			// landed in the window the user was being taken out of: what they actually saw
			// was the terminal move on its own and not one word about why. And the old
			// wording ("Focused the running supervisor") answered a question nobody asked
			// — someone who types `gtmux hq` is asking for a supervisor, so the useful
			// news is that one is already running, where it is, and that starting a second
			// is not something gtmux will do.
			i18n.Say("HQ is already running at "+where+". No new one was started; taking you there.",
				"HQ 已经在跑了（"+where+"），没有新建，这就把你切过去。")
			if err := panefocus.FocusPaneByID(pane); err != nil {
				i18n.Say("Could not switch to it. Go there with:  gtmux focus "+pane,
					"没能自动切过去，手动跳：  gtmux focus "+pane)
				return 0
			}
			// …and say it again where the user LANDS, because that is where their eyes are.
			noteAtPane(pane, i18n.Tr("gtmux: this is your HQ. It was already running, so nothing new was started",
				"gtmux：这就是你的 HQ，它本来就在跑，所以没有新建"))
			return 0
		}
		// Stamped but dead → relaunch the agent in the same pane, then focus. Reuse the
		// remembered choice (resolveHQLaunchAgent) so a revive doesn't silently fall back to
		// claude after the user picked another agent.
		rawCmd := hqLaunchOptions(resolveHQLaunchAgent(agentCmd))
		nameHQWindow(pane, false)
		diag.Did("act.hq.start", pane, diag.OK, "restarted HQ in the window it had", "agent", rawCmd, "how", "revive")
		i18n.Say("HQ had quit; restarting it in the window it already had ("+where+").",
			"HQ 之前退出了，正在它原来的窗口里重新拉起（"+where+"）。")
		// Reviving the pane we are sitting in: gtmux cannot type into a terminal it is
		// holding itself (briefhere.go), so it becomes the agent instead.
		if startsInOwnPane(pane) {
			return startHQHere(pane, rawCmd, "")
		}
		_ = tmux.SendText(pane, agentenv.Launch(rawCmd), true)
		_ = panefocus.FocusPaneByID(pane)
		noteAtPane(pane, i18n.Tr("gtmux: your HQ had quit, and it is restarted here",
			"gtmux：你的 HQ 之前退出了，已在这里重新拉起"))
		deliverHQBriefing(pane, rawCmd)
		return 0
	}

	// Spawn: detached session in the hq home, type the agent command (the same
	// mechanism restore/adopt use), then open a terminal tab onto it.
	name, err := tmux.Run(append(newSessionArgs(hqSessionName), "-c", state.HQHome())...)
	if err != nil || name == "" {
		name, err = tmux.Run("new-session", "-d", "-P", "-F", "#{session_name}", "-c", state.HQHome())
	}
	if err != nil || name == "" {
		diag.Did("act.hq.start", "", diag.Failed, "no tmux session could be created for HQ", "error", err)
		i18n.Sae("failed to create the HQ tmux session", "创建 HQ 的 tmux session 失败")
		return 1
	}
	// Resolve the agent to launch: --agent flag › GTMUX_HQ_AGENT › remembered choice ›
	// interactive picker of the installed agents (at a TTY) › claude. Fixes HQ silently
	// starting claude on a machine only signed into Codex.
	rawCmd := hqLaunchOptions(resolveHQLaunchAgent(agentCmd))
	// Auto-apply the network proxy so the agent starts correctly on whatever
	// network the user is on (home VPN vs office intranet) — no manual toggling.
	cmd := agentenv.Launch(rawCmd)
	pane := tmux.Display(name, "#{pane_id}")
	if pane != "" {
		nameHQWindow(pane, true)
		// Mark the pane as HQ before anything else can look for it: the stamp is the
		// one identity that survives a `cd` and needs no path comparison at all (a
		// symlinked config dir used to make every wake resolve "no HQ" silently).
		hqpane.Stamp(pane)
		_ = tmux.SendText(pane, cmd, true)
	}
	diag.Did("act.hq.start", pane, diag.OK, "started HQ in a new tmux session", "agent", rawCmd, "session", name, "how", "new")
	i18n.Say("HQ started in tmux session '"+name+"'.", "HQ 已在 tmux session '"+name+"' 启动。")
	if hostplatform.Current().DesktopFocus {
		term := terminal.Active()
		if _, err := term.SpawnTabs([]string{name}, false); err != nil {
			i18n.Sae("could not open a "+term.Name()+" tab; attach with:  "+hqAttachCommand(name),
				"无法打开 "+term.Name()+" tab，请手动接回：  "+hqAttachCommand(name))
		}
	} else {
		i18n.Say("attach with:  "+hqAttachCommand(name), "接回：  "+hqAttachCommand(name))
	}
	// Kick off the supervisor's FIRST turn: a self-introduction + fleet status report.
	// Runs only on a fresh spawn (a focused live HQ returned above), reuses the verified
	// dispatch path, and never fails `gtmux hq` if it can't land. (rawCmd, not the
	// proxy-wrapped cmd, so hook detection sees the bare agent name.)
	deliverHQBriefing(pane, rawCmd)
	return 0
}

// hqBriefingEnabled reports whether the startup briefing is on (the default). Set
// GTMUX_HQ_BRIEF to off/0/false/no to spawn HQ silently — for a user who prefers a
// quiet start, or who drives the first prompt themselves.
func hqBriefingEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GTMUX_HQ_BRIEF"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// hqBriefingPrompt is the MINIMAL, agent-agnostic trigger for HQ's first turn. The
// briefing procedure is linked from the seeded entry (AGENTS.md),
// which every agent reads through its own convention file (Claude via CLAUDE.md→
// AGENTS.md; Codex reads AGENTS.md natively). A one-line signal submits
// reliably where the old huge multi-line paste was flaky (typed-but-not-submitted), and
// ANY agent — not just Claude Code — acts on it per its playbook. It reuses the wake
// signal register (`» gtmux·<class>`) the playbook already teaches.
func hqBriefingPrompt() string {
	return i18n.Tr(
		"» gtmux·startup │ your first turn — produce your STARTUP BRIEFING per AGENTS.md.",
		"» gtmux·startup │ 你的第一轮 —— 按 AGENTS.md 产出你的启动简报。")
}

// deliverHQBriefing types the startup briefing into a freshly-spawned HQ pane so its
// first turn is a self-introduction + fleet status report. It reuses the verified
// dispatch path (wait-for-ready, then a land-verified deliver) — the same one
// `gtmux spawn` uses. Best-effort and non-fatal: a no-op when the pane is empty or the
// briefing is disabled, and a delivery that doesn't land never fails `gtmux hq` (the
// session is already up and usable — the user can simply type to it).
func deliverHQBriefing(pane, agentCmd string) {
	if pane == "" || !hqBriefingEnabled() {
		return
	}
	tune := dispatch.LoadTuning()
	if !dispatchbridge.WaitAgentReady(pane, agentCmd, time.Duration(tune.ReadyTimeout)*time.Second) {
		blocker, _ := dispatchbridge.ReadyBlocker(pane, agentCmd)
		diag.Did("act.hq.brief", pane, diag.Failed, "HQ's startup briefing never had a composer to land in",
			"error", blocker, "agent", agentCmd)
		return
	}
	// Non-fatal, but not SILENT. A refusal here has a cause the operator can act on — most
	// plainly a draft already sitting in the box — and swallowing it leaves an HQ that
	// simply never briefs, with nothing on screen to explain why.
	res := deliverHQBriefingInput(dispatchbridge.DispatchIO(pane),
		dispatchbridge.DeliverOpts(pane, agentCmd, false, tune), hqBriefingPrompt())
	diag.Did("act.hq.brief", pane, briefOutcome(res), "delivered HQ's startup briefing",
		"state", string(res.State), "judgedBy", res.JudgedBy, "how", "inline",
		"failureCode", string(res.FailureCode), "submission", string(res.Submission))
	switch {
	case res.State == dispatch.StateQueued:
		i18n.Say("  the startup briefing is queued; HQ runs it after the turn it is in",
			"  启动简报已排队，HQ 手上这轮结束后就会执行")
	case !res.Delivered:
		i18n.Sae("gtmux hq: the startup briefing was not delivered ("+string(res.State)+"); type to the pane to start it",
			"gtmux hq: 启动简报未送达（"+string(res.State)+"），直接在该 pane 里说话即可开始")
	}
}

func hqWhere(pane string) string {
	loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	if loc == "" {
		return pane
	}
	return loc + " · " + pane
}

// noteAtPane puts one line on tmux's status bar in the pane the user was just sent to.
//
// A command that moves you somewhere has a reporting problem: whatever it printed is
// back in the window you left, so the explanation and the reader end up in different
// places. This says it again where the reader is now. Best-effort in every direction —
// an older tmux without `-d` gets the plain form, and a failure changes nothing about
// what the command did.
func noteAtPane(pane, msg string) {
	if tmux.Bin == "" || pane == "" {
		return
	}
	if _, err := tmux.Run("display-message", "-d", "4000", "-t", pane, msg); err == nil {
		return
	}
	_, _ = tmux.Run("display-message", "-t", pane, msg)
}

// printBoard writes the supervisor's situation board to stdout.
//
// A board that has never been written is an ORDINARY state, not an error: a fresh HQ has
// none, and a reader is told that rather than shown a failure.
func printBoard(asJSON bool) int {
	text, mod, ok := Board()
	if asJSON {
		out, err := json.Marshal(struct {
			Exists    bool   `json:"exists"`
			UpdatedAt int64  `json:"updated_at,omitempty"`
			Text      string `json:"text,omitempty"`
		}{Exists: ok, UpdatedAt: mod, Text: text})
		if err != nil {
			return 1
		}
		fmt.Println(string(out))
		return 0
	}
	if !ok {
		i18n.Say("no situation board yet; HQ writes one as it works",
			"还没有态势板，HQ 干着干着就会写一份")
		return 0
	}
	fmt.Print(text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Println()
	}
	return 0
}

// printHQHome writes the HQ home path — the directory a `gtmux knowledge` mutation has to
// run from — to stdout.
//
// Same reason `--board` exists (#909): a surface that acts on HQ's memory must not have to
// know where that memory lives. The home is relocatable and on some machines is reached
// through a symlink, so ONE resolver answers for every consumer, and the menu bar stays a
// pure CLI consumer rather than a second implementation of the path.
//
// This does NOT widen the cwd-keyed role gate by one inch: the caller still has to run the
// verb from the home, and `gtmux knowledge` still judges it by cwd exactly as before. The
// path is public knowledge — every refusal message already prints it.
//
// A missing home still prints the path (a caller wants it for the message it will show)
// but exits non-zero, because chdir'ing there is about to fail and the reason is worth
// saying once, here, in words.
func printHQHome() int {
	home := state.HQHome()
	fmt.Println(home)
	if fi, err := os.Stat(home); err != nil || !fi.IsDir() {
		i18n.Sae("gtmux hq: no HQ home at "+home+" yet; run `gtmux hq` to create one",
			"gtmux hq: "+home+" 还不存在，先跑 `gtmux hq` 建一个")
		return 1
	}
	return 0
}

// exportMemoryCmd writes the supervisor's memory to one file.
//
// A tarball, not a format of gtmux's own: the one case this exists for is the case where
// gtmux may not be there to read it back.
func exportMemoryCmd(dst string, plain, passStdin bool) int {
	if plain {
		n, err := ExportMemory(dst)
		if err != nil {
			i18n.Sae("gtmux hq --export: "+err.Error(), "gtmux hq --export："+err.Error())
			return 1
		}
		recordExport(false)
		st := ReadMemoryState()
		i18n.Say(fmt.Sprintf("✓ wrote %s (%s, %d files), not encrypted (--plain)", dst, humanBytes(n), st.Files),
			fmt.Sprintf("✓ 已写入 %s（%s，%d 个文件），未加密（--plain）", dst, humanBytes(n), st.Files))
		i18n.Say("  It carries your project detail and whatever you told HQ to remember. Keep it where you would keep working notes.",
			"  里面是你的项目细节和你让 HQ 记住的事。按工作笔记的标准找地方放。")
		return 0
	}
	pass, err := askPassphrase(true, passStdin)
	if err != nil {
		i18n.Sae("gtmux hq --export: "+err.Error(), "gtmux hq --export："+err.Error())
		return 1
	}
	path, n, err := ExportMemoryEncrypted(dst, pass)
	if err != nil {
		i18n.Sae("gtmux hq --export: "+err.Error(), "gtmux hq --export："+err.Error())
		return 1
	}
	st := ReadMemoryState()
	i18n.Say(fmt.Sprintf("✓ wrote %s (%s, %d files), locked with your passphrase", path, humanBytes(n), st.Files),
		fmt.Sprintf("✓ 已写入 %s（%s，%d 个文件），已用你的口令上锁", path, humanBytes(n), st.Files))
	if path != dst {
		i18n.Say("  (.age added to the name: that is what the file is)", "  （文件名补了 .age：它就是这种文件）")
	}
	i18n.Say("  Open it with `gtmux hq --import "+path+"`, or any age tool. Lose the passphrase and it stays shut, because gtmux keeps no copy.",
		"  用 `gtmux hq --import "+path+"` 或任何 age 工具解开。口令丢了就打不开，gtmux 不留副本。")
	return 0
}

// importMemoryCmd restores one, never overwriting in place. An encrypted export asks for
// its passphrase first; a wrong one changes nothing.
func importMemoryCmd(src string, passStdin bool, expected string) int {
	if err := memoryWriteAllowed(); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	pass := ""
	if IsEncryptedArchive(src) {
		var err error
		pass, err = askPassphrase(false, passStdin)
		if err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		}
	}
	archive, err := readMemoryArchive(src, pass)
	if err == nil && expected != "" && archive.digest != expected {
		err = errors.New("archive changed since preview; preview it again")
	}
	if errors.Is(err, ErrWrongPassphrase) {
		i18n.Sae("gtmux hq --import: wrong passphrase, no records were changed", "gtmux hq --import：档案口令不正确，当前内容未修改。")
		return 1
	}
	if err != nil {
		i18n.Sae("gtmux hq --import: "+err.Error(), "gtmux hq --import："+err.Error())
		return 1
	}
	moved, err := restoreMemoryArchive(archive)
	if err != nil {
		i18n.Sae("gtmux hq --import: "+err.Error(), "gtmux hq --import："+err.Error())
		return 1
	}
	i18n.Say("✓ restored HQ's records from "+src, "✓ 已从 "+src+" 还原 HQ 的档案")
	if moved != "" {
		i18n.Say("  the records that were here are at "+moved+"; delete them once you are sure",
			"  原来那份档案挪到了 "+moved+"，确认无误后再删")
	}
	i18n.Say("  restart HQ so it reads the restored board and knowledge base.",
		"  重启 HQ，让它读到还原后的态势板与知识库。")
	return 0
}

// askPassphrase gets the passphrase from the one place the caller said, in this order:
// the first line of stdin (--passphrase-stdin, how the menu-bar app hands it over — never
// argv, where ps would print it), the GTMUX_HQ_PASSPHRASE variable (scripts), else the
// terminal, unechoed. For an export it is typed twice and must be PassphraseMin long;
// a mismatch or a short one asks again rather than failing, three tries. With no terminal
// and no other source there is nothing to ask, and the error says which flag to use.
func askPassphrase(confirm, fromStdin bool) (string, error) {
	if fromStdin {
		p, err := readPassphraseLine(os.Stdin)
		if err != nil {
			return "", err
		}
		if confirm && PassphraseStrength(p) == "short" {
			return "", fmt.Errorf("passphrase too short (%d characters at least)", PassphraseMin)
		}
		return p, nil
	}
	if p := os.Getenv("GTMUX_HQ_PASSPHRASE"); p != "" {
		if confirm && PassphraseStrength(p) == "short" {
			return "", fmt.Errorf("GTMUX_HQ_PASSPHRASE is too short (%d characters at least)", PassphraseMin)
		}
		return p, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New(i18n.Tr("no passphrase: pass --passphrase-stdin, set GTMUX_HQ_PASSPHRASE, or run from a terminal (--plain exports without one)",
			"没有口令：用 --passphrase-stdin、设 GTMUX_HQ_PASSPHRASE，或在终端里运行（--plain 可不加口令导出）"))
	}
	if !confirm {
		fmt.Fprint(os.Stderr, i18n.Tr("Passphrase: ", "口令："))
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	for tries := 0; tries < 3; tries++ {
		fmt.Fprint(os.Stderr, i18n.Tr(fmt.Sprintf("Passphrase for the export (%d+ characters, not echoed): ", PassphraseMin),
			fmt.Sprintf("给导出文件设一个口令（%d 位以上，不回显）：", PassphraseMin)))
		first, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if PassphraseStrength(string(first)) == "short" {
			i18n.Sae(fmt.Sprintf("  too short: %d characters at least", PassphraseMin), fmt.Sprintf("  太短：至少 %d 位", PassphraseMin))
			continue
		}
		fmt.Fprint(os.Stderr, i18n.Tr("Once more: ", "再输一次："))
		second, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			i18n.Sae("  they differ, try again", "  两次不一样，再输一次")
			continue
		}
		if PassphraseStrength(string(first)) == "good" {
			i18n.Sae("  good passphrase", "  口令强度：好")
		}
		return string(first), nil
	}
	return "", errors.New(i18n.Tr("no passphrase after three tries", "试了三次都没有口令"))
}

// humanBytes is a size a person reads, not a byte count.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
