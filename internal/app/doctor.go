package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/servermode"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// check status levels.
const (
	stOK   = iota // ✓ good
	stRec         // ⚠ recommended (a feature degrades without it)
	stMiss        // ✗ blocking (a core feature breaks without it)
	stInfo        // · neutral note
)

// dcheck is one rendered row: a status, a short label, the current value, and a
// dim one-line "why it matters" note.
type dcheck struct {
	status             int
	label, value, note string
}

// dsection groups related checks under a heading.
type dsection struct {
	title string
	rows  []dcheck
}

func statusGlyph(status int) (glyph, color string) {
	switch status {
	case stRec:
		return "⚠", i18n.Yellow
	case stMiss:
		return "✗", i18n.Red
	case stInfo:
		return "·", i18n.Dim
	default:
		return "✓", i18n.Green
	}
}

// cmdDoctor implements `gtmux doctor`: a grouped, READ-ONLY health check (Layer 1)
// mapping each gtmux feature to the tmux / terminal / hook prerequisite it needs.
// With `--fix` it then walks the recommended fixes, explaining and confirming each
// one (Layer 2, see doctorFix); `--yes` applies them all without prompting.
func cmdDoctor(args []string) int {
	fix, yes := false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			usage()
			return 0
		case "--fix":
			fix = true
		case "-y", "--yes":
			yes = true
		}
	}

	secs := doctorSections()

	fmt.Printf("%sgtmux doctor%s %s· macOS environment (read-only)%s\n\n",
		i18n.Bold, i18n.Reset, i18n.Dim, i18n.Reset)
	ok, rec, miss := renderSections(secs)

	fmt.Printf("  %s%d %s%s · %s%d %s%s · %s%d %s%s\n",
		i18n.Green, ok, i18n.Tr("ok", "正常"), i18n.Reset,
		i18n.Yellow, rec, i18n.Tr("to improve", "待改进"), i18n.Reset,
		i18n.Red, miss, i18n.Tr("blocking", "阻塞"), i18n.Reset)

	if fix {
		fmt.Println()
		return doctorFix(yes)
	}
	if rec > 0 || miss > 0 {
		// Don't make the user re-run from scratch with --fix: if we're on a TTY,
		// offer to walk the fixes right here (each step still explains + asks).
		// Off a TTY (piped / CI), just print the hint and keep doctor read-only.
		if isTTY() {
			fmt.Println()
			if confirm(i18n.Tr("  Fix these now? [Y/n] ", "  现在就修复这些？[Y/n] ")) {
				fmt.Println()
				return doctorFix(false)
			}
			i18n.Say("  → or run `gtmux doctor --fix` anytime (explains + asks before each change)",
				"  → 也可随时跑 `gtmux doctor --fix`（每步先解释并征求确认）")
		} else {
			i18n.Say("  → run `gtmux doctor --fix` to set up the rest (it explains and asks before each change)",
				"  → 跑 `gtmux doctor --fix` 把其余项配好（每步都会解释并征求确认）")
		}
	}
	if miss > 0 {
		return 1
	}
	return 0
}

// renderSections prints the grouped checks with aligned columns and returns the
// tally (ok / recommended / blocking).
func renderSections(secs []dsection) (ok, rec, miss int) {
	lw, vw := 0, 0
	for _, s := range secs {
		for _, r := range s.rows {
			if w := i18n.DispWidth(r.label); w > lw {
				lw = w
			}
			if w := i18n.DispWidth(r.value); w > vw {
				vw = w
			}
		}
	}
	for _, s := range secs {
		fmt.Printf("  %s%s%s\n", i18n.Bold, s.title, i18n.Reset)
		for _, r := range s.rows {
			switch r.status {
			case stOK:
				ok++
			case stRec:
				rec++
			case stMiss:
				miss++
			}
			glyph, color := statusGlyph(r.status)
			note := ""
			if r.note != "" {
				note = "   " + i18n.Dim + r.note + i18n.Reset
			}
			fmt.Printf("    %s%s%s  %s  %s%s\n",
				color, glyph, i18n.Reset,
				i18n.PadRight(r.label, lw),
				i18n.PadRight(r.value, vw), note)
		}
		fmt.Println()
	}
	return ok, rec, miss
}

// doctorSections runs every check and groups the rows by concern.
func doctorSections() []dsection {
	now := time.Now().Unix()
	agents := []dcheck{rowClaudeHook()}
	// Only surface Codex when it's actually present (~/.codex exists), so users
	// who don't run Codex aren't shown an irrelevant row.
	if fileExists(filepath.Join(homeDir(), ".codex")) {
		agents = append(agents, rowCodexHook())
	}
	secs := []dsection{
		// The gtmux install itself, FIRST — CLI + menu-bar app versions (a drift is the
		// first thing you see) + config validity.
		{i18n.Tr("gtmux", "gtmux"), versionChecks()},
		{i18n.Tr("tmux", "tmux"), []dcheck{rowTmux(), rowLocale(), rowSetTitles(),
			rowWindowNameSource(), rowPaneIDsInTabs(), rowPaneTitles(), rowHyperlinks(), rowHistory()}},
		{i18n.Tr("Restore after reboot", "重启后恢复"), restoreRebootChecks()},
		{i18n.Tr("Terminal", "终端"), terminalChecks()},
		{i18n.Tr("Agents & notifications", "agent 与通知"), append(agents, rowStaleBindings())},
		// The menu-bar app is its own concern — install state + version + on-disk path.
		{i18n.Tr("Menu-bar app", "菜单栏 app"), appChecks()},
		{i18n.Tr("Remote access", "远程访问"), remoteChecks()},
		{i18n.Tr("Storage", "存储"), []dcheck{rowDiskUsage(), rowUploads()}},
	}
	// Only for a machine that actually runs a supervisor — on any other install these
	// rows would report a cadence for a thing that does not exist.
	if fileExists(state.HQHome()) {
		secs = append(secs, dsection{i18n.Tr("HQ", "中控"), hqChecks(now)})
	}
	return secs
}

// hqConsumptionCheck reports whether HQ is keeping up with the event stream — the
// observability half of the consumption watermark (hq-watermark-wakes). Perception is
// silent by design in BOTH directions: a knock leaves no trace on any screen the user
// reads, and so does a knock that never lands. Before this row, "HQ has consumed nothing
// for two hours" was indistinguishable from "nothing happened for two hours", and the only
// detector was the commander noticing that a finished job went unremarked — which is
// exactly how the 2026-08-01 miss surfaced.
func hqConsumptionCheck(now int64) dcheck {
	label := i18n.Tr("event consumption", "事件消费")
	c := hq.ConsumptionStatus(now)
	switch c.State {
	case hq.MaintenanceNever:
		return dcheck{stInfo, label, i18n.Tr("no watermark yet", "尚无水位"),
			i18n.Tr("HQ has not pulled the stream yet — expected before its first wake",
				"中控还没拉过事件流 —— 首次唤醒前属正常")}
	case hq.MaintenanceSlipped:
		value := strconv.Itoa(c.Unread) + i18n.Tr(" behind", " 条未消费")
		if c.StandingSec > 0 {
			value += " · " + hq.HumanAgeShort(c.StandingSec)
		}
		return dcheck{stRec, label, value,
			i18n.Tr("HQ is not consuming what it is knocked about — check the HQ pane for a stuck draft",
				"中控没在消费敲给它的事件 —— 检查中控窗格输入框是否卡住")}
	default:
		if c.Unread == 0 {
			return dcheck{stOK, label, i18n.Tr("caught up", "已跟上"),
				i18n.Tr("HQ has read the stream through its end", "中控已读到事件流末尾")}
		}
		return dcheck{stOK, label, strconv.Itoa(c.Unread) + i18n.Tr(" behind", " 条未消费"),
			i18n.Tr("a normal in-flight delta — it re-knocks until consumed",
				"正常在途增量 —— 未消费会持续敲门")}
	}
}

// hqSessionHealthCheck reports whether the supervisor's own session is still fit to judge
// (hq-self-rotate). Every other row in this section asks about HQ's products; this one asks
// about HQ. It exists because the degradation it names — a long, near-full session starting
// to read its OWN output as input that came from outside — cannot be self-detected, so
// without a row here the only detector left is the commander noticing that HQ believed
// something he never said. That is exactly how it surfaced on 2026-08-03.
func hqSessionHealthCheck(now int64) dcheck {
	label := i18n.Tr("HQ session health", "中控会话健康")
	h := hq.SessionHealthStatus(now)
	switch h.State {
	case hq.MaintenanceNever:
		// No live HQ, or no session recorded for it yet. Informational, NOT a warning: an
		// absent supervisor is not a degraded one, and a doctor that flagged every machine
		// with an HQ home and no HQ running would be ignored on the day it was right.
		return dcheck{stInfo, label, i18n.Tr("no live HQ", "中控未在运行"),
			i18n.Tr("nothing to judge — start one with `gtmux hq`", "无可判读 —— `gtmux hq` 可启动")}
	case hq.MaintenanceSlipped:
		// The FIGURES stay in the value column (same shape as the healthy row, so the two
		// are comparable at a glance) and the crossed threshold leads the note — it is the
		// reason, and a reader who disputes it needs to see which line was crossed.
		return dcheck{stRec, label, h.Figures(),
			i18n.Tr("over "+h.Over+" — HQ should bring its board + knowledge base current, "+
				"hand off, then `gtmux hq --rotate`",
				"越线 "+h.Over+" —— 中控应把态势板与知识库写到最新、交接后 `gtmux hq --rotate`")}
	default:
		return dcheck{stOK, label, h.Figures(),
			i18n.Tr("context, age and turn count are all under their rotation thresholds",
				"上下文占用、会话时长与轮数都在轮换阈值以内")}
	}
}

// hqMaintenanceChecks reports whether HQ's two periodic rituals are actually running.
// They are raised by the resident serve process, so a stalled cadence means something
// systemic (serve down, the HQ pane unresolvable, a wedged sensor) — and it is otherwise
// invisible: both passes are silent by design, so "it has not distilled in three weeks"
// looks exactly like "nothing needed distilling". This row is the difference.
func hqMaintenanceChecks(now int64) []dcheck {
	distill, selfCheck := hq.MaintenanceStatus(now)
	return []dcheck{
		maintenanceRow(distill,
			i18n.Tr("knowledge distill", "知识蒸馏"),
			i18n.Tr("weekly pass folds the fleet's lessons into the knowledge base",
				"每周把舰队的教训沉淀进知识库"),
			i18n.Tr("no distill for over a week+grace — is `gtmux serve` running with a live HQ?",
				"超过一周+宽限没有蒸馏 —— `gtmux serve` 还在跑、中控还活着吗？")),
		maintenanceRow(selfCheck,
			i18n.Tr("HQ self-check", "中控自检"),
			i18n.Tr("daily pass checks ledger / feed / memory health", "每日自检账本、感知与记忆健康"),
			i18n.Tr("no self-check for over a day+grace — is `gtmux serve` running with a live HQ?",
				"超过一天+宽限没有自检 —— `gtmux serve` 还在跑、中控还活着吗？")),
		promotionsRow(hq.PromotionsStatus(now)),
	}
}

// promotionsRow reports the knowledge export queue (hq-promotion-exit): quiet when
// empty or young, flagged once the oldest pending brief has stood past its floor —
// a promotion nobody carries is silent rot, and this row is where it stops being
// silent.
func promotionsRow(r hq.PromotionsRow) dcheck {
	label := i18n.Tr("knowledge promotions", "知识晋升队列")
	switch {
	case r.Pending == 0:
		return dcheck{stOK, label, i18n.Tr("queue clear", "队列已清空"),
			i18n.Tr("no charter-level lesson is waiting to land", "没有待落地的守则级教训")}
	case r.State == hq.MaintenanceSlipped:
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("%d pending · oldest %s", "%d 条待落地 · 最久 %s"),
				r.Pending, hq.HumanAgeShort(r.OldestSec)),
			i18n.Tr("a brief has waited past its floor — land it in its carrier (a project AGENTS.md, a runbook, LOCAL.md, or a gtmux issue), then `gtmux knowledge land <id> --ref …`",
				"有简报滞留超期 —— 请落到它的载体(项目 AGENTS.md、runbook、LOCAL.md 或 gtmux issue),再用 `gtmux knowledge land <id> --ref …` 闭环")}
	default:
		return dcheck{stOK, label,
			fmt.Sprintf(i18n.Tr("%d pending · oldest %s", "%d 条待落地 · 最久 %s"),
				r.Pending, hq.HumanAgeShort(r.OldestSec)),
			i18n.Tr("briefs under knowledge/promotions/ await landing", "knowledge/promotions/ 下的简报等待落地")}
	}
}

// maintenanceRow renders one cadence verdict. A pass inside its floor is OK; one in the
// grace window is a neutral note (the zero-change gate legitimately skips quiet periods);
// past that it is a ⚠, because the cadence itself has stopped.
func maintenanceRow(r hq.MaintenanceRow, label, okNote, slipNote string) dcheck {
	switch r.State {
	case hq.MaintenanceNever:
		return dcheck{stInfo, label, i18n.Tr("never run", "从未运行"),
			i18n.Tr("no pass raised yet — expected on a fresh HQ", "尚未触发过 —— 新装中控属正常")}
	case hq.MaintenanceSlipped:
		return dcheck{stRec, label, hq.HumanAgeShort(r.AgeSec) + i18n.Tr(" ago", "前"), slipNote}
	default:
		return dcheck{stOK, label, hq.HumanAgeShort(r.AgeSec) + i18n.Tr(" ago", "前"), okNote}
	}
}

// diskAmberBytes / diskRedBytes are the state-dir footprint thresholds the storage
// sentinel warns at. Disk hygiene (diskHygieneSweep) keeps a healthy install well under
// 100 MB; amber flags one trending large, red flags one an unrotated log or a stuck
// hygiene pass has let balloon (the multi-GB incident this guards against).
const (
	diskAmberBytes int64 = 500 << 20 // 500 MB
	diskRedBytes   int64 = 2 << 30   // 2 GB
)

// rowDiskUsage reports the total gtmux state-dir footprint and flags a runaway — the
// visible half of disk hygiene: the sweep keeps it bounded, this makes a breach legible
// before the disk does. An unrotated daemon log (serve/tunnel.log) is the usual culprit.
func rowDiskUsage() dcheck {
	label := i18n.Tr("gtmux disk usage", "gtmux 磁盘占用")
	total := treeSize(state.Dir())
	switch {
	case total >= diskRedBytes:
		return dcheck{stMiss, label, humanBytes(total),
			i18n.Tr("very large — check for a runaway log (serve/tunnel.log)",
				"过大 —— 检查是否有失控日志（serve/tunnel.log）")}
	case total >= diskAmberBytes:
		return dcheck{stRec, label, humanBytes(total),
			i18n.Tr("trending large — hygiene trims logs/uploads automatically",
				"偏大 —— hygiene 会自动裁剪日志/上传")}
	default:
		return dcheck{stOK, label, humanBytes(total),
			i18n.Tr("state dir under control", "状态目录占用正常")}
	}
}

// --- individual checks (read-only) ---

func rowTmux() dcheck {
	if tmux.Bin == "" {
		return dcheck{stMiss, i18n.Tr("tmux", "tmux"), i18n.Tr("not found", "未找到"),
			i18n.Tr("gtmux needs tmux — brew install tmux", "gtmux 依赖 tmux，brew install tmux")}
	}
	ver := ""
	if v := tmux.Lines("-V"); len(v) > 0 {
		ver = strings.TrimPrefix(v[0], "tmux ")
	}
	note := ""
	if newer := brewOutdatedVersion("tmux"); newer != "" {
		note = i18n.Tr("newer available: "+newer+" — brew upgrade tmux",
			"有新版 "+newer+" —— brew upgrade tmux")
	}
	return dcheck{stOK, i18n.Tr("tmux", "tmux"), ver, note}
}

// rowLocale flags when the environment's locale isn't UTF-8. Without a UTF-8
// LC_CTYPE/LANG, tmux substitutes every non-ASCII byte with "_"/"?" — so CJK
// (中文) file names render as ? and the ✳/braille agent glyphs `classifyAgent`
// keys off get mangled. gtmux forces UTF-8 on its OWN tmux calls (internal/tmux),
// but panes and shells you open inherit the ambient env, so a non-UTF-8 locale
// still bites your interactive `ls` and any pane gtmux didn't spawn.
func rowLocale() dcheck {
	label := i18n.Tr("locale", "字符集")
	note := i18n.Tr("UTF-8 so 中文 names + agent glyphs render right",
		"UTF-8 才能正确显示中文名称与 agent 图标")
	cs := localeCharset()
	if isUTF8Locale(cs) {
		return dcheck{stOK, label, cs, note}
	}
	val := cs
	if val == "" {
		val = i18n.Tr("unset", "未设置")
	}
	return dcheck{stRec, label, val,
		i18n.Tr("not UTF-8 — 中文 file names show as ?; set a UTF-8 LANG",
			"非 UTF-8——中文文件名显示为 ?；需设置 UTF-8 的 LANG")}
}

// localeCharset returns the effective locale string in POSIX precedence
// (LC_ALL > LC_CTYPE > LANG), or "" when none is set.
func localeCharset() string {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// isUTF8Locale reports whether a locale string selects the UTF-8 charset.
func isUTF8Locale(v string) bool {
	u := strings.ToUpper(v)
	return strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8")
}

func rowSetTitles() dcheck {
	label := i18n.Tr("set-titles", "set-titles")
	note := i18n.Tr("focus/restore locate tabs by this title", "focus/restore 靠此标题定位 tab")
	if tmuxOpt("set-titles") == "on" && tmuxOpt("set-titles-string") == "#S — #W" {
		return dcheck{stOK, label, "on · '#S — #W'", note}
	}
	return dcheck{stMiss, label, i18n.Tr("not set", "未设置"), note}
}

// titleSaysSomething reports whether a pane's title tells you anything about the work in
// it — the question the pane browser silently answers every time it falls back to the
// command name.
//
// Measured on a real fleet: of four plain shell panes, one title was `:/Users/ccy`, one was
// empty, one was the machine's own host name, and the fourth was the same. All four rows
// therefore read `bash`, while the agent pane beside them read 提炼本周研发周报质量部分汇总 —
// because the agent WRITES its title and a shell does not. So the three junk shapes are:
// empty, a path (many prompts set the title to the cwd), and the command itself.
//
// The host name is already stripped upstream by the pane producer, so it arrives empty.
func titleSaysSomething(title, command string) bool {
	t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(title), ":"))
	if t == "" || t == strings.TrimSpace(command) {
		return false
	}
	return !strings.HasPrefix(t, "/") && !strings.HasPrefix(t, "~")
}

// rowPaneTitles reports how many PLAIN panes can say what they are for.
//
// Agent panes are excluded on purpose: an agent sets its own title, so counting them would
// report a healthy fleet made entirely of rows the user cannot tell apart. This measures
// the OUTCOME (what the panes' titles actually are) rather than looking for a hook in a
// shell rc, which can be written a hundred ways.
func rowPaneTitles() dcheck {
	label := i18n.Tr("pane titles", "pane 标题")
	note := i18n.Tr("a plain pane says what it is for, not just \"bash\"", "普通 pane 能说出自己在干嘛，而不只是一个 \"bash\"")
	var plain, named int
	for _, p := range radar.GatherPanes() {
		if p.Tier == "agent" {
			continue
		}
		plain++
		if titleSaysSomething(p.Title, p.Command) {
			named++
		}
	}
	// Whether the hook is INSTALLED changes what an unnamed pane means. It only titles
	// shells started after it, so right after `--fix` writes it every existing pane is
	// still unnamed — and the post-fix summary, which re-reads these rows and lists any
	// that are still "to improve", told the user their fix had not worked seconds after
	// it did. An old pane is not something to go fix; it is something that ages out.
	installed := shellHookInstalled()
	switch {
	case plain == 0:
		return dcheck{stInfo, label, i18n.Tr("no plain panes", "没有普通 pane"), note}
	case named == plain:
		return dcheck{stOK, label, i18n.Tr("all named", "都有名字"), note}
	case installed:
		return dcheck{stInfo, label,
			fmt.Sprintf(i18n.Tr("%d of %d named · hook installed", "%d/%d 有名字 · 钩子已装"), named, plain),
			i18n.Tr("panes opened before the hook keep their old titles", "钩子之前开的 pane 保持原标题")}
	case named > 0:
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("%d of %d named", "%d/%d 有名字"), named, plain), note}
	default:
		return dcheck{stRec, label,
			fmt.Sprintf(i18n.Tr("none of %d named", "%d 个都没有名字"), plain), note}
	}
}

// shellHookInstalled reports whether the pane-title hook is in the startup file a tmux
// pane sources. Checked by MARKER, not by behaviour — the behaviour it produces cannot be
// observed in panes that were opened before it existed, which is the whole point.
func shellHookInstalled() bool {
	rc, _ := shellRCPath()
	b, err := os.ReadFile(rc)
	return err == nil && strings.Contains(string(b), paneTitleMarker)
}

// hyperlinkFeature is the terminal-features flag that lets tmux pass OSC 8 hyperlinks
// through to the outer terminal. Without it tmux DROPS them: it only writes a hyperlink
// for a terminal it believes supports one, and no terminal claims the capability by
// default. Measured on this machine — `terminal-features` listed clipboard/cstyle/focus/
// title and no hyperlinks, and clicking a link in a tmux pane did nothing.
const hyperlinkFeature = ",*:hyperlinks"

// minHyperlinkTmux is the first tmux that KNOWS this feature name. `terminal-features`
// arrived in 3.2, but `hyperlinks` only in 3.4 — writing it into an older config is not a
// no-op, it is a startup error on a line the user did not write themselves.
const minHyperlinkTmux = "3.4"

// tmuxVersion returns the running tmux's version ("3.7b" → "3.7b"), or "" when unknown.
func tmuxVersion() string {
	v := tmux.Lines("-V")
	if len(v) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(v[0], "tmux "))
}

// tmuxAtLeast compares a tmux version against a "major.minor" floor, tolerating the
// letter suffix tmux uses for its release candidates and point releases ("3.7b", "3.4a").
// Unknown or unparseable → false: a feature is offered only when it is known to exist.
func tmuxAtLeast(ver, floor string) bool {
	num := func(s string) (int, int) {
		var maj, min int
		f := strings.SplitN(s, ".", 2)
		maj, _ = strconv.Atoi(strings.TrimFunc(f[0], func(r rune) bool { return r < '0' || r > '9' }))
		if len(f) > 1 {
			min, _ = strconv.Atoi(strings.TrimFunc(f[1], func(r rune) bool { return r < '0' || r > '9' }))
		}
		return maj, min
	}
	vMaj, vMin := num(ver)
	fMaj, fMin := num(floor)
	if vMaj == 0 {
		return false // unknown
	}
	return vMaj > fMaj || (vMaj == fMaj && vMin >= fMin)
}

// rowHyperlinks reports whether a clickable link in a pane actually reaches the terminal.
func rowHyperlinks() dcheck {
	label := i18n.Tr("clickable links", "可点击的链接")
	note := i18n.Tr("a link printed in a pane is clickable", "pane 里打印的链接可以点开")
	ver := tmuxVersion()
	if !tmuxAtLeast(ver, minHyperlinkTmux) {
		return dcheck{stInfo, label,
			i18n.Tr("needs tmux "+minHyperlinkTmux+"+", "需要 tmux "+minHyperlinkTmux+"+"),
			i18n.Tr("this tmux has no hyperlinks feature — brew upgrade tmux",
				"当前 tmux 没有 hyperlinks 能力 —— brew upgrade tmux")}
	}
	if strings.Contains(tmuxOptAll("terminal-features"), "hyperlinks") {
		return dcheck{stOK, label, i18n.Tr("on", "已开"), note}
	}
	return dcheck{stRec, label, i18n.Tr("tmux drops them", "被 tmux 丢弃"),
		i18n.Tr("tmux only forwards a link to a terminal that claims the capability",
			"tmux 只会把链接转发给声明了该能力的终端")}
}

// paneIDsInWindowName is the automatic-rename-format that puts every pane's `%N` into its
// window's name — and therefore into the terminal TAB, since set-titles-string is
// `#S — #W` (tmux-id-surface phase 2).
//
// Why the window NAME and not set-titles-string: the tab then carries the ids without
// gtmux touching the title format, so the ghostty/iTerm2 title MATCHERS need no change —
// they prefix-match on the session and the ids land after the `#S — ` separator. That
// coupling is the risk this design avoids on purpose: `tab-alert`'s `● ` prefix broke
// every jump on 2026-08-13 for exactly that reason.
const paneIDsInWindowName = "#{b:pane_current_path} #{P:#{pane_id} }"

// paneIDsRefreshHook forces the name to be recomputed when a pane CLOSES.
//
// Measured 2026-08-14 on an isolated server: adding a pane re-evaluates
// automatic-rename-format immediately, but REMOVING one does not — the name kept a dead
// `%1` while `%0 %2` were the live panes. Toggling the option off and on inside a
// `pane-exited` hook forces the recompute, and it stays correct across consecutive kills.
// So this hook is REQUIRED for the format to stay true, not a nicety.
const paneIDsRefreshHook = "set-window-option automatic-rename off ; set-window-option automatic-rename on"

// rowPaneIDsInTabs reports whether the terminal tab can name the panes it is showing.
//
// RECOMMENDED, never required: without it every gtmux surface still works — this is about
// being able to read a tab and know which panes are behind it, which is the "global sense"
// tmux-id-surface exists for. gtmux does NOT write it: `automatic-rename-format` is the
// user's own configuration, and on the machine this was designed against it was already
// set to something better than gtmux would have guessed.
func rowPaneIDsInTabs() dcheck {
	label := i18n.Tr("pane ids in tab titles", "标签页标题带 pane id")
	note := i18n.Tr("read a tab and know which panes it holds", "看一眼标签就知道它有哪些 pane")
	fmtOpt := tmuxOpt("automatic-rename-format")
	hasIDs := strings.Contains(fmtOpt, "#{pane_id}")
	hasHook := strings.Contains(tmuxHook("pane-exited"), "automatic-rename")
	switch {
	case hasIDs && hasHook:
		// Report what the WINDOWS say, not what the option says. Measured on a real fleet
		// minutes after the option was applied: 2 of 12 windows carried ids. The rest had
		// `automatic-rename` OFF — tmux turns it off for any window that was ever renamed
		// by hand, and the format simply does not reach those. A row that reads ✓ while
		// ten tabs show nothing is the same defect as comparing a format to a literal:
		// it checks the setting instead of the outcome.
		named, total, manual := windowsNamingTheirPanes()
		switch {
		case total == 0:
			return dcheck{stOK, label, i18n.Tr("on · refresh hook set", "已开 · 刷新 hook 已设"), note}
		case named == total:
			return dcheck{stOK, label, i18n.Tr("on · every window", "已开 · 每个窗口都带"), note}
		case manual > 0:
			return dcheck{stInfo, label,
				fmt.Sprintf(i18n.Tr("%d/%d windows (%d named by hand)", "%d/%d 个窗口（%d 个是你手动命名的）"), named, total, manual),
				i18n.Tr("a hand-renamed window keeps its name — gtmux will not rename it back",
					"手动命名过的窗口保持原名 —— gtmux 不会替你改回去")}
		default:
			return dcheck{stRec, label,
				fmt.Sprintf(i18n.Tr("%d/%d windows", "%d/%d 个窗口"), named, total),
				i18n.Tr("the rest refresh on their next activity", "其余窗口下次有动静时会刷新")}
		}
	case hasIDs:
		// The half-configured case is worth calling out separately: the ids are there but
		// go stale on a pane close, which is worse than not having them — a name that
		// lists a pane which no longer exists.
		return dcheck{stRec, label, i18n.Tr("on, but no pane-exited hook (ids go stale)",
			"已开，但缺 pane-exited hook（关 pane 后会过期）"), note}
	default:
		return dcheck{stRec, label, i18n.Tr("not set", "未设置"), note}
	}
}

// windowNameFollowsCommand reports whether a window's name is derived from its foreground
// COMMAND — the shape that renders a Claude pane as its version string.
//
// It cannot compare against a literal default: tmux 3.7's real default is
// `#{?pane_in_mode,[tmux],#{pane_current_command}}#{?pane_dead,[dead],}`, not the bare
// `#{pane_current_command}` this first tested for — so every default install was reported
// as "custom, left alone" and never saw the suggestion. Measured, not assumed; and asking
// what the format IS BUILT FROM survives tmux decorating its default again.
func windowNameFollowsCommand(format string) bool {
	return format == "" || strings.Contains(format, "pane_current_command")
}

// windowsNamingTheirPanes counts how many windows actually carry a pane id in their name,
// and how many CANNOT because they were renamed by hand (`automatic-rename` off — tmux
// turns it off permanently for a window the moment someone renames it).
//
// That distinction is the whole point of reporting it: "8 of 12" with no reason reads as a
// bug, while "8 of 12, 4 named by hand" is a complete answer the user can act on.
func windowsNamingTheirPanes() (named, total, manual int) {
	for _, line := range tmux.Lines("list-windows", "-a", "-F", "#{automatic-rename}\t#{window_name}") {
		f := strings.SplitN(line, "\t", 2)
		if len(f) < 2 {
			continue
		}
		total++
		if f[0] == "0" {
			manual++
			continue
		}
		if strings.Contains(f[1], "%") {
			named++
		}
	}
	return named, total, manual
}

// rowWindowNameSource reports what a window name is made of.
//
// tmux's DEFAULT is `#{pane_current_command}`, which for a Claude pane renders its VERSION
// STRING — `2.1.229`, the #659 fact — so a default user's tab reads `gtmux dev — 2.1.229`.
// That is the "window names drift" problem in its real form, and the fix is a one-line
// format the user owns, not gtmux renaming their windows (which would turn
// `automatic-rename` off and overwrite whatever they had).
func rowWindowNameSource() dcheck {
	label := i18n.Tr("window-name source", "窗口名来源")
	fmtOpt := strings.TrimSpace(tmuxOpt("automatic-rename-format"))
	switch {
	case windowNameFollowsCommand(fmtOpt):
		return dcheck{stRec, label, i18n.Tr("the foreground command (an agent shows its version)",
			"前台命令（agent 会显示成版本号）"),
			i18n.Tr("prefer the directory: #{b:pane_current_path}", "建议改用目录名：#{b:pane_current_path}")}
	default:
		return dcheck{stOK, label, fmtOpt, i18n.Tr("custom — left alone", "自定义 —— 不动它")}
	}
}

func rowHistory() dcheck {
	label := i18n.Tr("history-limit", "history-limit")
	hl := tmuxOpt("history-limit")
	if v, _ := strconv.Atoi(hl); v >= 10000 {
		return dcheck{stOK, label, hl, i18n.Tr("scrollback depth", "滚动缓冲深度")}
	}
	return dcheck{stRec, label, hl, i18n.Tr("raise to ~50000 for deeper snapshots", "调到 ~50000 获得更深快照")}
}

func rowPlugins() []dcheck {
	specs := []struct{ dir, label, en, zh string }{
		{"tpm", i18n.Tr("TPM", "TPM"), "plugin manager", "插件管理器"},
		{"tmux-resurrect", i18n.Tr("tmux-resurrect", "tmux-resurrect"), "restores layout after reboot", "重启后恢复布局"},
		{"tmux-continuum", i18n.Tr("tmux-continuum", "tmux-continuum"), "auto-save + auto-restore", "自动存档 + 恢复"},
	}
	rows := make([]dcheck, 0, len(specs))
	for _, s := range specs {
		if pluginDir(s.dir) != "" {
			rows = append(rows, dcheck{stOK, s.label, i18n.Tr("installed", "已装"), i18n.Tr(s.en, s.zh)})
		} else {
			rows = append(rows, dcheck{stRec, s.label, i18n.Tr("missing", "未装"), i18n.Tr(s.en, s.zh)})
		}
	}
	return rows
}

func rowCapture() dcheck {
	label := i18n.Tr("capture-pane", "capture-pane")
	note := i18n.Tr("snapshot scrollback on restore", "restore 时带回 scrollback")
	if tmuxOpt("@resurrect-capture-pane-contents") == "on" {
		return dcheck{stOK, label, "on", note}
	}
	return dcheck{stRec, label, "off", note}
}

// restoreRebootChecks assembles the "Restore after reboot" rows. The autosave-armed
// check is appended only when continuum is installed AND a server is up (its trigger
// lives in the running status-right) — otherwise there's nothing meaningful to read.
func restoreRebootChecks() []dcheck {
	rows := append(rowPlugins(), rowCapture(), rowAutoRestore())
	if pluginDir("tmux-continuum") != "" && tmux.ServerUp() {
		rows = append(rows, rowAutoSave())
	}
	return rows
}

// rowAutoSave verifies continuum's autosave is actually armed: its save trigger must be
// in the running status-right, else continuum never saves and a reboot restores an
// ancient snapshot (the exact silent failure the restore-save-reliability change guards).
func rowAutoSave() dcheck {
	label := i18n.Tr("resurrect autosave", "resurrect 自动保存")
	note := i18n.Tr("continuum must save periodically for a reboot to restore",
		"continuum 需周期保存,重启才能恢复")
	switch n := continuumTriggerCount(tmuxOpt("status-right")); {
	case n == 1:
		// Armed is a CLAIM. continuum's trigger only runs while tmux redraws the status
		// bar, which needs an awake machine with a terminal attached — so a laptop that
		// sleeps can carry a perfectly-configured trigger and save nothing for hours (76
		// such gaps in 3.5 days here, the longest just under six). Report what the file
		// says, not what the config says.
		if age, ok := saveAge(resurrectLastSave(), time.Now()); ok && age >= backstopArmedStaleAfter {
			return dcheck{stRec, label,
				i18n.Tr("armed, but idle "+hq.HumanAgeShort(int64(age.Seconds())),
					"已装,但 "+hq.HumanAgeShort(int64(age.Seconds()))+" 没存过"),
				i18n.Tr("the trigger is in status-right but nothing has saved for a long while — continuum only fires while the status bar redraws, so a sleeping Mac saves nothing. `gtmux serve` backstops it; if that is not running, your layout is only as fresh as the age shown",
					"触发器在 status-right 里,但已经很久没存过了 —— continuum 只在状态栏重画时才跑,Mac 一睡就不存。`gtmux serve` 会兜底;若没在跑,你的存档就只有这个新鲜度")}
		}
		return dcheck{stOK, label, i18n.Tr("installed", "已装"), note}
	case n > 1:
		// Two triggers = every interval runs the save twice, forever, silently. It comes
		// from a path-FORM mismatch: continuum looks for its own ABSOLUTE path, so a
		// hand-written `~/…` trigger doesn't match and it appends a second copy.
		return dcheck{stRec, label,
			i18n.Tr(fmt.Sprintf("%d triggers", n), fmt.Sprintf("%d 个触发器", n)),
			i18n.Tr("status-right carries continuum's save trigger more than once — every save runs that many times. Usually a `~/…` trigger you wrote by hand plus the absolute-path one continuum added because `~` didn't match its check; keep ONE (the absolute form)",
				"status-right 里有多份 continuum 保存触发器 —— 每次保存会跑这么多遍。通常是你手写的 `~/…` 那份 + continuum 因为 `~` 没匹配上它的检查而又追加的绝对路径那份;只保留一份(用绝对路径那份)")}
	}
	return dcheck{stRec, label, i18n.Tr("trigger missing", "触发器缺失"),
		i18n.Tr("status-right lacks continuum's save trigger — autosave is OFF; append #(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh) to status-right",
			"status-right 缺少 continuum 保存触发器 —— 自动保存已关;在 status-right 末尾追加 #(~/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)")}
}

func rowAutoRestore() dcheck {
	label := i18n.Tr("auto-restore", "auto-restore")
	note := i18n.Tr("continuum restores on tmux start", "tmux 启动时 continuum 自动恢复")
	if tmuxOpt("@continuum-restore") == "on" {
		return dcheck{stOK, label, "on", note}
	}
	return dcheck{stRec, label, "off", note}
}

func rowTerminal() dcheck {
	name := terminal.DetectedName()
	label := i18n.Tr("host", "宿主终端")
	if name == "warp" {
		// Warp has no per-tab scripting: restore/new work (launch configs), focus
		// lands on the app (exact tab only for gtmux-opened tabs). Be honest here.
		return dcheck{stOK, label, name, i18n.Tr("restore / new supported; focus is best-effort (Warp has no tab scripting)",
			"restore / new 可用；focus 尽力而为（Warp 无 tab 脚本接口）")}
	}
	if terminal.HasDriver(name) {
		return dcheck{stOK, label, name, i18n.Tr("focus / restore / new supported", "focus / restore / new 可用")}
	}
	return dcheck{stRec, label, name, i18n.Tr("no driver — agents work, focus/restore don't", "暂无驱动，agents 照常，focus/restore 不可用")}
}

func rowClaudeHook() dcheck {
	label := i18n.Tr("Claude Code hook", "Claude Code hook")
	if claudeHookInstalled() {
		return dcheck{stOK, label, i18n.Tr("installed", "已装"), i18n.Tr("⏸ needs-input + notifications", "⏸ 需要输入 + 通知")}
	}
	return dcheck{stRec, label, i18n.Tr("not installed", "未装"), i18n.Tr("⏸ needs-input + notifications", "⏸ 需要输入 + 通知")}
}

func rowCodexHook() dcheck {
	label := i18n.Tr("Codex hook", "Codex hook")
	// Installed via the preferred hooks system (precise state), or the legacy notify.
	if codexHooksWired() {
		if codexHooksStale() {
			// Present but marked async, which Codex 0.146.0 skips — it reports "installed"
			// yet never fires. A real fix, not a note.
			return dcheck{stRec, label, i18n.Tr("stale (async — Codex skips it)", "陈旧（async —— Codex 会跳过）"),
				i18n.Tr("run `gtmux doctor --fix` to reinstall as sync (else it never fires)", "跑 `gtmux doctor --fix` 重装为 sync（否则永不触发）")}
		}
		return dcheck{stOK, label, i18n.Tr("installed", "已装"), i18n.Tr("precise state + notifications", "状态精准 + 通知")}
	}
	if codexNotifyIsGtmux() {
		return dcheck{stOK, label, i18n.Tr("installed (notify)", "已装（notify）"), i18n.Tr("turn-done notifications", "turn 结束通知")}
	}
	// Only reached when ~/.codex EXISTS (this row isn't added otherwise), so Codex is
	// in use — an un-wired hook is a real improvement (`--fix` offers it), not a
	// neutral note. Detection still works without it, but you miss precise per-event
	// state + notifications.
	return dcheck{stRec, label, i18n.Tr("not installed", "未装"), i18n.Tr("install for precise state + notifications", "接入以获精准状态 + 通知")}
}

// rowCloudflared surfaces the optional tunnel client. It's only needed for
// `gtmux tunnel` (remote phone access), so a missing one is neutral (·), not a
// problem — `doctor --fix` offers to install it.
func rowCloudflared() dcheck {
	label := i18n.Tr("cloudflared", "cloudflared")
	if lookTool("cloudflared") != "" {
		return dcheck{stOK, label, i18n.Tr("installed", "已装"),
			i18n.Tr("remote access via `gtmux tunnel`", "`gtmux tunnel` 远程访问")}
	}
	return dcheck{stInfo, label, i18n.Tr("not installed", "未装"),
		i18n.Tr("optional — only for `gtmux tunnel`", "可选，仅 `gtmux tunnel` 需要")}
}

// --- gtmux install (version + config) ---

// versionChecks is the FIRST section: the gtmux install itself. CLI + menu-bar app
// versions (so a drift between them is the first thing you see — after the recent
// CLI-ahead-of-app confusion), plus config validity.
func versionChecks() []dcheck {
	cli := strings.TrimPrefix(Version, "v")
	rows := []dcheck{{stOK, i18n.Tr("cli", "命令行"), "v" + cli, cliPathNote()}}
	app := installedAppVersion()
	switch {
	case app == "":
		rows = append(rows, dcheck{stInfo, i18n.Tr("app", "app"), i18n.Tr("not installed", "未装"),
			i18n.Tr("menu-bar app not installed — `gtmux update`", "菜单栏 app 未装 —— `gtmux update`")})
	case app == cli:
		rows = append(rows, dcheck{stOK, i18n.Tr("app", "app"), "v" + app,
			i18n.Tr("menu-bar app matches the CLI", "菜单栏 app 与 CLI 一致")})
	default:
		rows = append(rows, dcheck{stRec, i18n.Tr("app", "app"), "v" + app,
			i18n.Tr("menu-bar app is behind the CLI (v"+cli+") — run `gtmux update`",
				"菜单栏 app 落后于 CLI（v"+cli+"）—— 跑 `gtmux update`")})
	}
	return append(rows, rowConfig())
}

// cliPathNote is the on-disk location of the running gtmux binary (falls back to the
// standard install path if the executable path can't be resolved).
func cliPathNote() string {
	if p, err := os.Executable(); err == nil && p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".local", "bin", "gtmux")
}

// rowConfig validates ~/.config/gtmux/config.json parses — a malformed config is silently
// ignored (defaults used), so a typo can quietly drop every setting.
func rowConfig() dcheck {
	label := i18n.Tr("config", "配置")
	path := filepath.Join(homeDir(), ".config", "gtmux", "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return dcheck{stInfo, label, i18n.Tr("defaults", "默认"),
			i18n.Tr("no config.json — all defaults", "无 config.json —— 全用默认")}
	}
	if !json.Valid(b) {
		return dcheck{stRec, label, i18n.Tr("invalid JSON", "JSON 有误"),
			i18n.Tr("config.json isn't valid JSON — settings are ignored; fix or delete it",
				"config.json 不是合法 JSON —— 设置被忽略；修好或删掉")}
	}
	return dcheck{stOK, label, i18n.Tr("valid", "合法"), "~/.config/gtmux/config.json"}
}

// --- terminal detection ---

// knownTerminals: the terminals gtmux recognizes, and whether it can DRIVE each
// (focus/restore/new) via a registered driver. A sensed-only terminal hosts tmux + agents
// fine; gtmux just can't focus/restore it.
var knownTerminals = []struct{ name, bundle, driver string }{
	{"Ghostty", "Ghostty.app", "ghostty"},
	{"iTerm2", "iTerm.app", "iterm2"},
	{"Apple Terminal", "Terminal.app", ""},
	{"kitty", "kitty.app", ""},
	{"WezTerm", "WezTerm.app", ""},
	{"Alacritty", "Alacritty.app", ""},
	{"Warp", "Warp.app", "warp"},
}

func terminalChecks() []dcheck { return []dcheck{rowTerminal(), rowOtherTerminals()} }

// terminalInstalled reports whether a terminal .app bundle is present in the usual dirs.
func terminalInstalled(bundle string) bool {
	dirs := []string{"/Applications", filepath.Join(homeDir(), "Applications")}
	if bundle == "Terminal.app" { // Apple Terminal ships in the system utilities
		dirs = append(dirs, "/System/Applications/Utilities", "/Applications/Utilities")
	}
	for _, d := range dirs {
		if fileExists(filepath.Join(d, bundle)) {
			return true
		}
	}
	return false
}

// rowOtherTerminals lists terminals installed BESIDES the current host, each marked
// supported (has a gtmux driver → focus/restore/new work) or sensed-only.
func rowOtherTerminals() dcheck {
	label := i18n.Tr("other terminals", "其它终端")
	host := terminal.DetectedName()
	var parts []string
	for _, t := range knownTerminals {
		if t.driver != "" && t.driver == host {
			continue // the host is the row above
		}
		if !terminalInstalled(t.bundle) {
			continue
		}
		if t.driver == "warp" && terminal.HasDriver(t.driver) {
			parts = append(parts, t.name+i18n.Tr(" (best-effort)", "（尽力支持）"))
		} else if t.driver != "" && terminal.HasDriver(t.driver) {
			parts = append(parts, t.name+i18n.Tr(" (supported)", "（支持）"))
		} else {
			parts = append(parts, t.name+i18n.Tr(" (sensed)", "（仅感知）"))
		}
	}
	if len(parts) == 0 {
		return dcheck{stInfo, label, i18n.Tr("none", "无"),
			i18n.Tr("only the host terminal is installed", "只装了宿主终端")}
	}
	return dcheck{stInfo, label, strconv.Itoa(len(parts)), strings.Join(parts, " · ")}
}

// --- menu-bar app (its own section) ---

// appChecks is the menu-bar app as its own concern: install state + version + on-disk
// path, and whether it's current with the CLI (the version-inconsistency the recent update
// fix now resolves).
func appChecks() []dcheck {
	path := gtmuxAppPath()
	if _, err := os.Stat(path); err != nil {
		alt := "/Applications/Gtmux.app" // Homebrew cask default
		if _, e2 := os.Stat(alt); e2 != nil {
			return []dcheck{{stRec, i18n.Tr("installed", "安装"), i18n.Tr("not installed", "未装"),
				i18n.Tr("needed for desktop notifications — `gtmux update`", "桌面通知需要它 —— `gtmux update`")}}
		}
		path = alt
	}
	app := installedAppVersion()
	ver := i18n.Tr("unknown", "未知")
	if app != "" {
		ver = "v" + app
	}
	rows := []dcheck{
		{stOK, i18n.Tr("installed", "安装"), ver, i18n.Tr("delivers desktop notifications", "负责发桌面通知")},
		{stOK, i18n.Tr("path", "路径"), path, i18n.Tr("on-disk bundle", "磁盘上的 bundle")},
	}
	cli := strings.TrimPrefix(Version, "v")
	if app != "" && cli != "" && app != cli {
		rows = append(rows, dcheck{stRec, i18n.Tr("up to date", "是否最新"), i18n.Tr("behind CLI", "落后 CLI"),
			i18n.Tr("app v"+app+" < CLI v"+cli+" — `gtmux update` (now refreshes a stale app)",
				"app v"+app+" < CLI v"+cli+" —— `gtmux update`（现在能更新滞后的 app）")})
	}
	return rows
}

// --- remote access ---

func remoteChecks() []dcheck {
	rows := append([]dcheck{rowCloudflared()}, sleepSettingChecks()...)
	return append(rows, rowServeRunning(), rowLiveActivity())
}

// localServeGet reads a host-only endpoint off the serve running on this Mac. Kept
// small and local to doctor: it answers "what does the RUNNING process think", which no
// file on disk can (the Live Activity token is held in memory by design).
func localServeGet(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", defaultServePort, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+resolveServeToken(""))
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serve returned %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// rowLiveActivity reports whether this Mac can still update the phone's lock-screen
// card.
//
// The card is the one surface that FAILS INVISIBLY. Its relative times are rendered on
// the phone from the last pushed state, so when the Mac stops pushing, the card does not
// go blank or stale-looking — it keeps counting, and reads as a session that has been
// working for hours. Measured 2026-08-22: nearly three hours of that, while the phone
// was connected the whole time and nothing on either side said the push token was gone.
//
// The token lives in the running serve's memory, so this asks the serve rather than a
// file. No serve, or no push configured, is not a fault to report here — rowServeRunning
// covers the first and push is optional.
func rowLiveActivity() dcheck {
	label := i18n.Tr("lock-screen card", "锁屏卡片")
	note := i18n.Tr("the phone's Live Activity is updated by THIS Mac pushing to it",
		"手机锁屏那张卡片，是由本机推送更新的")
	body, err := localServeGet("/api/push/tokens")
	if err != nil {
		return dcheck{stInfo, label, i18n.Tr("serve not answering", "服务未响应"), note}
	}
	var payload struct {
		Activities []struct {
			Env          string `json:"env"`
			RegisteredAt int64  `json:"registeredAt"`
		} `json:"activities"`
		LastPush int64 `json:"activityLastPush"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return dcheck{stInfo, label, i18n.Tr("unreadable", "读不出"), note}
	}
	if len(payload.Activities) == 0 {
		return dcheck{stInfo, label, i18n.Tr("no card registered", "没有已注册的卡片"),
			i18n.Tr("nothing to update — either no Live Activity is running on the phone, or its push token never reached this Mac (open the app to re-register)",
				"没有可更新的对象 —— 要么手机上没有在跑的实时活动，要么它的推送 token 没送到本机（打开 app 会重新注册）")}
	}
	val := fmt.Sprintf(i18n.Tr("%d registered", "已注册 %d 个"), len(payload.Activities))
	if payload.LastPush > 0 {
		val += " · " + i18n.Tr("last push ", "上次推送 ") + hq.HumanAgeShort(time.Now().Unix()-payload.LastPush)
	}
	return dcheck{stOK, label, val, note}
}

// rowServeRunning reports whether a local gtmux serve is actually LISTENING (the phone's
// endpoint), not just whether cloudflared is installed — an installed tunnel with no serve
// behind it still can't answer the phone.
func rowServeRunning() dcheck {
	label := i18n.Tr("serve", "服务")
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", defaultServePort), 400*time.Millisecond)
	if err != nil {
		return dcheck{stInfo, label, i18n.Tr("not running", "未运行"),
			i18n.Tr("no local server on :8765 — the phone can't reach this Mac (start it from the menu-bar app or `gtmux awake`)",
				"本机 :8765 无服务 —— 手机连不上（用菜单栏 app 或 `gtmux awake` 开启）")}
	}
	_ = c.Close()
	return dcheck{stOK, label, i18n.Tr("running :8765", "运行中 :8765"),
		i18n.Tr("the phone can reach this Mac", "手机可连到本机")}
}

// --- storage: phone uploads ---

// rowUploads surfaces the phone image-upload staging dir (a picture the phone sends into a
// pane lands here first). It grows silently; this makes it visible + `doctor --fix` clears
// it. Neutral until it's large enough to bother clearing.
func rowUploads() dcheck {
	label := i18n.Tr("phone uploads", "手机上传")
	n, size := dirCountSize(uploadsDir())
	if n == 0 {
		return dcheck{stInfo, label, i18n.Tr("empty", "空"),
			i18n.Tr("images the phone sends into a pane stage here", "手机发进 pane 的图片暂存在此")}
	}
	note := uploadsDir() + i18n.Tr(" — `gtmux doctor --fix` clears it", " —— `gtmux doctor --fix` 可清理")
	val := fmt.Sprintf("%s (%d)", humanBytes(size), n)
	if size >= 100<<20 { // 100 MB — worth clearing
		return dcheck{stRec, label, val, note}
	}
	return dcheck{stInfo, label, val, note}
}

// uploadsDir is the phone image-upload staging dir (mirrors serve.go's saveUpload).
func uploadsDir() string { return filepath.Join(homeDir(), ".local", "share", "gtmux", "uploads") }

// dirCountSize returns the file count + total bytes of a directory tree (0,0 if absent).
func dirCountSize(dir string) (int, int64) {
	var n int
	var size int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		n++
		if info, e := d.Info(); e == nil {
			size += info.Size()
		}
		return nil
	})
	return n, size
}

// --- HQ (supervisor health, detailed) ---

// hqChecks reports the supervisor's own health in more detail than a single cadence line:
// its home, the situation board's freshness, the knowledge base's shape, and the
// maintenance cadences. Only assembled when an HQ home exists.
func hqChecks(now int64) []dcheck {
	rows := []dcheck{
		{stOK, i18n.Tr("home", "家目录"), i18n.Tr("present", "存在"), state.HQHome()},
		rowHQBoard(now),
		rowHQKnowledge(),
		hqConsumptionCheck(now),
		hqSessionHealthCheck(now),
	}
	return append(rows, hqMaintenanceChecks(now)...)
}

// rowHQBoard reports the situation board's freshness — HQ's persistent memory that
// survives context resets.
func rowHQBoard(now int64) dcheck {
	label := i18n.Tr("board", "看板")
	info, err := os.Stat(hq.BoardPath())
	if err != nil {
		return dcheck{stInfo, label, i18n.Tr("none yet", "尚无"),
			i18n.Tr("notes/board.md not written yet — HQ writes it as it works", "notes/board.md 尚未写入 —— HQ 干活时会写")}
	}
	val := hq.HumanAgeShort(now-info.ModTime().Unix()) + i18n.Tr(" ago · ", "前 · ") + humanBytes(info.Size())
	return dcheck{stOK, label, val, i18n.Tr("HQ's persistent situation board (survives resets)", "HQ 的常驻情况看板（跨重置保存）")}
}

// rowHQKnowledge reports the knowledge base's shape: curated topics + how much is queued
// for the next distill pass (the learning loop's inbox).
func rowHQKnowledge() dcheck {
	label := i18n.Tr("knowledge base", "知识库")
	kdir := filepath.Join(state.HQHome(), "knowledge")
	topics := 0
	entries, _ := os.ReadDir(kdir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			topics++
		}
	}
	if topics == 0 {
		return dcheck{stInfo, label, i18n.Tr("none", "无"),
			i18n.Tr("no knowledge topics yet — HQ distills lessons here", "尚无知识主题 —— HQ 会在此沉淀教训")}
	}
	pending := countNonEmptyLines(filepath.Join(kdir, ".pending-distill.jsonl"))
	val := fmt.Sprintf(i18n.Tr("%d topics", "%d 主题"), topics)
	note := fmt.Sprintf(i18n.Tr("curated lessons; %d pending distill", "沉淀的教训；%d 条待蒸馏"), pending)
	return dcheck{stOK, label, val, note}
}

// countNonEmptyLines counts non-blank lines in a file (0 if absent).
func countNonEmptyLines(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

// brewOutdatedVersion returns the newer version Homebrew has for a formula, or "" when
// it's current / not brew-managed / brew absent. `brew outdated --json=v2` reads the local
// formula cache (no network), so it degrades to "" on any error rather than blocking.
func brewOutdatedVersion(formula string) string {
	brew := lookTool("brew")
	if brew == "" {
		return ""
	}
	out, err := exec.Command(brew, "outdated", "--json=v2", formula).Output()
	if err != nil {
		return ""
	}
	var parsed struct {
		Formulae []struct {
			CurrentVersion string `json:"current_version"`
		} `json:"formulae"`
	}
	if json.Unmarshal(out, &parsed) != nil || len(parsed.Formulae) == 0 {
		return ""
	}
	return parsed.Formulae[0].CurrentVersion
}

// --- shared probes (also used by doctorFix) ---

// tmuxOpt reads a global tmux option's value ("" if unset/error).
func tmuxOpt(name string) string {
	lines := tmux.Lines("show", "-gv", name)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// tmuxOptAll is tmuxOpt for an ARRAY option. `show -gv terminal-features` prints one
// line per entry, and tmuxOpt keeps only the first — so a `set -as` append landed on a
// later line and read as absent. That made the hyperlinks step re-offer itself forever
// (caught by its idempotency check, not by reading the code).
func tmuxOptAll(name string) string {
	return strings.Join(tmux.Lines("show", "-gv", name), "\n")
}

// tmuxHook returns a hook's command, or "" when it is unset.
//
// It asks for the hook BY NAME. Measured: `show-hooks -g` without a name does not list a
// hook that has been set — it prints the known hook names — so scanning that output for
// one would report every configured hook as absent. With a name it prints
// `pane-exited[0] <command>`, so the value is what follows the first space.
func tmuxHook(name string) string {
	lines := tmux.Lines("show-hooks", "-g", name)
	if len(lines) == 0 {
		return ""
	}
	f := strings.SplitN(strings.TrimSpace(lines[0]), " ", 2)
	if len(f) < 2 {
		return ""
	}
	return strings.TrimSpace(f[1])
}

// pluginDir returns the install dir of a TPM plugin if present.
func pluginDir(name string) string {
	for _, base := range []string{
		filepath.Join(homeDir(), ".tmux", "plugins"),
		filepath.Join(homeDir(), ".config", "tmux", "plugins"),
	} {
		p := filepath.Join(base, name)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
	}
	return ""
}

// claudeHookInstalled reports whether ~/.claude/settings.json has a gtmux hook.
func claudeHookInstalled() bool {
	b, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	hooks, _ := m["hooks"].(map[string]any)
	for _, ev := range hooks {
		arr, _ := ev.([]any)
		for _, item := range arr {
			obj, _ := item.(map[string]any)
			inner, _ := obj["hooks"].([]any)
			for _, h := range inner {
				ho, _ := h.(map[string]any)
				if cmd, _ := ho["command"].(string); isGtmuxHookCommand(cmd) {
					return true
				}
			}
		}
	}
	return false
}

// codexNotifyIsGtmux reports whether Codex's notify points at gtmux.
func codexNotifyIsGtmux() bool {
	b, err := os.ReadFile(filepath.Join(homeDir(), ".codex", "config.toml"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "notify") && strings.Contains(l, "gtmux") && strings.Contains(l, "codex") {
			return true
		}
	}
	return false
}

// treeSize / humanBytes are small local copies of the disk-usage helpers the hq
// disk-hygiene sweep also uses — a leaf-level primitive duplicated (not shared)
// so the doctor command need not import the supervisor package.
func treeSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return strconv.FormatFloat(float64(n)/(1<<30), 'f', 1, 64) + " GB"
	case n >= 1<<20:
		return strconv.FormatInt(n>>20, 10) + " MB"
	case n >= 1<<10:
		return strconv.FormatInt(n>>10, 10) + " KB"
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}

// --- server mode (openspec change server-mode) ---

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

// sleepChecksFor is the pure decision half, so every branch is testable without
// touching the machine.
func sleepChecksFor(st servermode.Status, stale bool) []dcheck {
	if !st.SystemDisableSleep && !st.PersistedDisableSleep && !st.OwnedByGtmux {
		return nil // never used here — say nothing
	}
	label := i18n.Tr("sleep setting", "睡眠设置")
	manual := i18n.Tr("undo it with: sudo pmset -a disablesleep 0",
		"手动恢复：sudo pmset -a disablesleep 0")

	switch {
	case st.State == servermode.StateLapsed:
		return []dcheck{{stRec, label, i18n.Tr("lapsed", "已失效"),
			i18n.Tr("gtmux thinks server mode is on but the kernel disabled it — treat the closed-lid session as over",
				"gtmux 以为服务器模式开着，但内核已关闭 —— 请认为合盖会话已结束")}}

	case st.SystemDisableSleep && !st.OwnedByGtmux:
		return []dcheck{{stRec, label, i18n.Tr("this Mac will not sleep", "这台 Mac 不会睡眠"),
			i18n.Tr("not set by gtmux — reported only, never changed for you. ", "不是 gtmux 设的 —— 只报告、不代改。") + manual}}

	case st.SystemDisableSleep:
		// Ours. Fresh heartbeat = server mode in use; stale = left behind.
		if !stale {
			return []dcheck{{stOK, label, i18n.Tr("server mode on", "服务器模式开启中"),
				i18n.Tr("deliberate — the lid may close", "有意为之 —— 合盖不会睡")}}
		}
		return []dcheck{{stMiss, label, i18n.Tr("left disabled by gtmux", "被 gtmux 留在关闭睡眠状态"),
			i18n.Tr("no live server mode — this Mac cannot sleep. ", "没有在运行的服务器模式 —— 这台 Mac 无法睡眠。") + manual}}

	default:
		// Live sleep is fine but the persisted value would disable it again.
		return []dcheck{{stRec, label, i18n.Tr("persisted as disabled", "落盘为禁止睡眠"),
			i18n.Tr("sleeps now, but the stored setting would disable it after a reboot. ",
				"现在能睡，但落盘的设置会在重启后再次禁止。") + manual}}
	}
}

// staleBindingLead is how far ahead an UNBOUND session must be before the binding
// beside it is called stale. A few minutes of lead is normal: an agent that just
// started a new session fires its hook moments later and the binding catches up.
const staleBindingLead = 10 * 60

// staleBindingFresh bounds the row to a binding that is stale RIGHT NOW: the
// unclaimed session beside it must itself have been active within this window.
//
// Without it the row flags leftovers. Measured on a real fleet: pane %17 is bound to
// a session last spoken to 22 days ago and shares its directory with an unrelated
// session from two weeks before that — newer than the binding, owned by nobody, and
// completely inert. That is a folder with old files in it, not a pane whose chat is
// lying to its reader.
//
// The trade-off is deliberate and worth stating: a binding that went stale days ago
// and then went quiet is NOT reported, because at that point it is indistinguishable
// from the leftovers. What this row catches is the case that costs hours — a live
// conversation running beside a pane that is still serving the old one.
const staleBindingFresh = 2 * 60 * 60

// rowStaleBindings reports panes whose chat is being served from a log the agent
// has moved on from.
//
// On 2026-08-18 pane %13 spent 5h15m in exactly that state: Claude moved the
// conversation into a background session host, the hook lost its $TMUX_PANE and was
// filed as a native session, and the pane's binding kept naming a log whose last
// message was five hours old. Nothing errored anywhere — the phone showed a calm,
// complete conversation and the radar showed a settled `idle`.
//
// paneidentity.go closes the way that binding went stale. This row exists because the
// SILENCE is the real defect: a binding can go stale for reasons we have not met yet,
// and a reader must never again be shown five-hour-old history as if it were current.
//
// The question it asks is the one that actually distinguishes the failure: does this
// pane's own session DIRECTORY hold a NEWER conversation that no pane is bound to? A
// first attempt compared the pane's tmux activity against its log instead, and flagged
// 13 of 13 panes on a real fleet — `window_activity` is a WINDOW's clock, so a
// neighbouring pane's output makes a month-idle agent look busy. An unclaimed newer
// session in the same directory is the evidence; anything else is a proxy for it.
func rowStaleBindings() dcheck {
	label := i18n.Tr("chat binding", "会话绑定")
	note := i18n.Tr("the chat is served from the log a pane is bound to",
		"手机/网页的对话正是从 pane 绑定的那份记录里读出来的")
	now := time.Now().Unix()

	type bound struct {
		pane string
		rec  resume.Record
		path string
		last int64
	}
	var bounds []bound
	claimed := map[string]bool{} // every session id some pane already owns
	for _, p := range radar.GatherPanes() {
		if p.Tier != "agent" || p.Loc == "" {
			continue
		}
		rec, ok := resume.Load(p.Loc)
		if !ok || rec.SessionID == "" {
			continue
		}
		claimed[rec.SessionID] = true
		path := transcript.LogPath(rec.Agent, rec.SessionID)
		if path == "" {
			continue
		}
		bounds = append(bounds, bound{pane: p.PaneID, rec: rec, path: path,
			last: transcript.LastMessageTime(rec.Agent, rec.SessionID)})
	}

	var stale []string
	for _, b := range bounds {
		if b.last == 0 {
			continue // a log we cannot read says nothing about the binding
		}
		newer := newestUnclaimedSession(b.path, b.rec, claimed, b.last)
		if newer > b.last+staleBindingLead && now-newer < staleBindingFresh {
			stale = append(stale, fmt.Sprintf("%s (%s)", b.pane, hq.HumanAgeShort(now-b.last)))
		}
	}
	switch {
	case len(bounds) == 0:
		return dcheck{stInfo, label, i18n.Tr("nothing bound yet", "暂无绑定"), note}
	case len(stale) == 0:
		return dcheck{stOK, label,
			fmt.Sprintf(i18n.Tr("%d live", "%d 个在跟"), len(bounds)), note}
	default:
		return dcheck{stRec, label, strings.Join(stale, " · "),
			i18n.Tr("a newer conversation sits beside these panes' logs and no pane owns it — their chat is showing old history",
				"这些 pane 的记录旁边躺着更新的对话、而且没有任何 pane 认领 —— 它们的对话页显示的是旧历史")}
	}
}

// newestUnclaimedSession returns the last-message time of the newest session in
// `boundPath`'s directory that no pane is bound to. Agent-agnostic by construction:
// the directory and the file extension come from the pane's OWN log, so whatever
// layout an agent uses is the layout we search.
//
// Bounded on purpose — the mtime pre-filter means a directory of old sessions costs
// one stat each, and only a genuinely newer file is ever parsed.
func newestUnclaimedSession(boundPath string, rec resume.Record, claimed map[string]bool, boundLast int64) int64 {
	dir := filepath.Dir(boundPath)
	ext := filepath.Ext(boundPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var newest int64
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ext {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ext)
		if id == rec.SessionID || claimed[id] {
			continue
		}
		path := filepath.Join(dir, e.Name())
		// Filter against the bound log's last MESSAGE, never against its mtime. The
		// file that started all this had the NEWER mtime of the two — Claude appended a
		// `permission-mode` record to the dead log at 23:09 while the live conversation
		// had moved on at 17:47. An mtime pre-filter therefore skipped the very
		// candidate it existed to find, and this row stayed green through the exact
		// failure it was written for. mtime is a claim; the last message is the fact.
		if radar.FileMtime(path) < boundLast {
			continue // cheap stat filter before any parse
		}
		if t := transcript.LastMessageTime(rec.Agent, id); t > newest {
			newest = t
		}
	}
	return newest
}
