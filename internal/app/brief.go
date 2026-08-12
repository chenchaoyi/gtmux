package app

// `gtmux brief` — the supervisor's report to the commander, rendered BY GTMUX and
// written where the commander will actually see it in colour.
//
// WHY THIS EXISTS. HQ's prose to the commander goes through the terminal's
// GitHub-flavoured markdown renderer, which knows bold, code fences and literal
// characters — and cannot emit colour at all. It once used `&nbsp;` for indentation and
// the commander got a screenshot's worth of visible garbage. Meanwhile `gtmux digest`
// already renders a colour-aligned table, but it renders into HQ's TOOL RESULT, which the
// commander does not read; HQ has to retype it as markdown, and the colour dies in that
// retyping. The capability existed and simply could not cross the HQ layer.
//
// WHICH CHANNEL, AND WHY NOT THE OBVIOUS ONE. The wake channel (`internal/hqnudge`) looks
// like the answer — it already writes into a pane — but it is an INPUT channel: it types
// through `paste-buffer` / `send-keys -l` into the agent's composer and submits. Escape
// codes on that path arrive as LITERAL characters: mangled text on the commander's screen
// and a garbage user message for HQ. That is the `&nbsp;` mistake again, one layer down —
// mistaking a channel meant for an agent to READ for one meant for a human to SEE.
//
// The display channel is the pane's own tty (`#{pane_tty}`). Verified live before this was
// written: Claude Code reports `alternate_on = 0`, so it does not own an alternate screen
// and external writes land in the normal scroll flow rather than being wiped by a
// full-screen redraw; a colour probe written to a live pane's tty came back out of
// `capture-pane -e` with its SGR intact.
//
// COLOUR DISCIPLINE (the commander's 2026-08-02 rule, and the reason this file is fussy
// about it): RED IS FOR DECISIONS ONLY. This machine sits near 90% disk more or less
// permanently, and a line that is permanently red is a line nobody reads. Machine numbers
// therefore render dim. Colour is valuable because it is scarce, not because it is
// applied.

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// briefInput is what HQ hands over. Structured, and read from STDIN rather than argv:
// this repo has already paid for prose-on-a-command-line once, when backticks inside a
// `--body "$(…)"` were executed by the shell and spawned a rogue serve.
type briefInput struct {
	// Grade is the charter's three-level register: decision / attention / ledger.
	Grade string `json:"grade"`
	// Headline is the one sentence a commander reads if they read nothing else.
	Headline string `json:"headline"`
	// Lines are the supporting detail, column-aligned by this renderer.
	Lines []briefLine `json:"lines,omitempty"`
	// Pane targets a specific pane; empty resolves the HQ pane, which is the point.
	Pane string `json:"pane,omitempty"`
}

type briefLine struct {
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
	// Tone is the line's own weight, INDEPENDENT of the brief's grade: `decision` (red —
	// this specific line needs the commander), `dim` (a machine number, present but not
	// competing), or empty for normal. A brief may be a decision overall while most of
	// its lines are context.
	Tone string `json:"tone,omitempty"`
}

// Brief grades and tones, as the JSON spells them.
const (
	briefDecision  = "decision"
	briefAttention = "attention"
	briefLedger    = "ledger"
	briefDim       = "dim"
)

// briefMarker prefixes every rendered brief.
//
// It is not decoration: the brief lands in HQ's own scrollback, so HQ's next screen read
// would otherwise find its own report and treat it as new input — the self-feeding loop
// this project has already met on the wake channel, where the unread count had to exclude
// HQ's own records for exactly this reason. A stable, greppable marker lets a reader
// recognise its own voice and skip it.
const briefMarker = "» gtmux·brief"

func cmdBrief(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			i18n.Say(briefUsage())
			return 0
		}
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		i18n.Sae("gtmux brief: expects a JSON brief on stdin — see `gtmux brief --help`",
			"gtmux brief: 需要从 stdin 读入 JSON —— 见 `gtmux brief --help`")
		return 2
	}
	var in briefInput
	if err := json.Unmarshal(raw, &in); err != nil {
		i18n.Sae("gtmux brief: malformed JSON: "+err.Error(),
			"gtmux brief: JSON 格式错误: "+err.Error())
		return 2
	}
	if strings.TrimSpace(in.Headline) == "" {
		i18n.Sae("gtmux brief: `headline` is required — a brief with no sentence is not a brief",
			"gtmux brief: 必须有 `headline` —— 没有那句话就不成其为简报")
		return 2
	}
	pane := in.Pane
	if pane == "" {
		if pane = hqpane.Find(); pane == "" {
			i18n.Sae("gtmux brief: no HQ pane resolved — pass `pane` to target one explicitly",
				"gtmux brief: 找不到 HQ pane —— 用 `pane` 明确指定")
			return 1
		}
	}
	tty := strings.TrimSpace(tmux.Display(pane, "#{pane_tty}"))
	if tty == "" {
		i18n.Sae("gtmux brief: pane "+pane+" has no tty (gone?)",
			"gtmux brief: pane "+pane+" 没有 tty（已消失？）")
		return 1
	}
	payload := renderBrief(in, true)
	// MID-TURN briefs are QUEUED, not written. A TUI repaints the region the cursor sits
	// in, so bytes written while the agent is rendering are wiped by its next frame —
	// measured: the same write survived on an idle pane and vanished on a rendering one.
	//
	// "Is it rendering?" is answered from the HOOK, not the screen: `active/<pane>` is
	// created on UserPromptSubmit and removed on Stop, so its existence IS mid-turn. That
	// file predicted both outcomes of the experiment exactly, and it costs one stat.
	//
	// It must not BLOCK until quiet, tempting as that is: HQ issues the brief inside its
	// own turn, and that turn cannot end until this tool call returns. Waiting here would
	// deadlock the very condition being waited for. So it queues, and the resident serve
	// flushes it — the same shape as the wake channel.
	if hq.PaneMidTurn(pane) {
		if err := hq.QueueBrief(pane, payload); err != nil {
			i18n.Sae("gtmux brief: queue failed: "+err.Error(),
				"gtmux brief: 入队失败: "+err.Error())
			return 1
		}
		i18n.Say("queued — it lands when this turn ends (a rendering pane would wipe it)",
			"已入队 —— 本轮回答结束后落屏（正在渲染的 pane 会把它擦掉）")
		return 0
	}
	if err := hq.WriteBriefTo(tty, payload); err != nil {
		i18n.Sae("gtmux brief: write failed: "+err.Error(),
			"gtmux brief: 写入失败: "+err.Error())
		return 1
	}
	return 0
}

// briefGlyph maps a grade to the charter's register glyph. An unknown grade reads as the
// quietest one: a brief that cannot say how loud it is has not earned loud.
func briefGlyph(grade string) string {
	switch grade {
	case briefDecision:
		return "◆"
	case briefAttention:
		return "▸"
	default:
		return "·"
	}
}

// briefHeadColor is the grade's colour. Only a DECISION is red — see the file header.
// Attention is cyan (the working hue, "something is moving"), ledger is dim.
func briefHeadColor(grade string) string {
	switch grade {
	case briefDecision:
		return i18n.Red
	case briefAttention:
		return i18n.Cyan
	default:
		return i18n.Dim
	}
}

// renderBrief produces the block of bytes written to the pane. Pure over its inputs so
// the layout is testable without a tmux server; `color` off yields the same text with no
// escapes, which is what the tests assert against for alignment.
func renderBrief(in briefInput, color bool) string {
	paint := func(style, s string) string {
		if !color || style == "" {
			return s
		}
		return style + s + i18n.Reset
	}
	var b strings.Builder
	b.WriteString("\r\n")
	b.WriteString(paint(briefHeadColor(in.Grade), briefMarker+" "+briefGlyph(in.Grade)))
	b.WriteString("  ")
	b.WriteString(paint(i18n.Bold, in.Headline))
	b.WriteString("\r\n")

	// Column width from the widest label, measured in DISPLAY cells: a CJK label is two
	// cells per rune, and padding by byte or rune count would ragged the column exactly
	// where this repo's users write.
	w := 0
	for _, l := range in.Lines {
		if d := i18n.DispWidth(l.Label); d > w {
			w = d
		}
	}
	for _, l := range in.Lines {
		pad := strings.Repeat(" ", max(0, w-i18n.DispWidth(l.Label)))
		style := ""
		switch l.Tone {
		case briefDecision:
			style = i18n.Red
		case briefDim:
			style = i18n.Dim
		}
		b.WriteString("  ")
		b.WriteString(paint(i18n.Dim, l.Label))
		b.WriteString(pad)
		b.WriteString("  ")
		b.WriteString(paint(style, l.Value))
		if l.Note != "" {
			b.WriteString("  ")
			b.WriteString(paint(i18n.Dim, l.Note))
		}
		b.WriteString("\r\n")
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func briefUsage() (string, string) {
	return `gtmux brief — write a colour-rendered report into the supervisor's pane

  Reads a JSON brief on STDIN and writes it, coloured and column-aligned, to the
  pane's tty — the channel a human SEES, unlike the wake channel, which types into
  the agent's composer for it to READ.

  echo '{"grade":"decision","headline":"two sessions need you",
         "lines":[{"label":"api","value":"waiting 11m","tone":"decision"},
                  {"label":"disk","value":"41GB free","tone":"dim"}]}' | gtmux brief

  grade   decision (◆, red) | attention (▸, cyan) | ledger (·, dim)
  tone    per line: decision (red) | dim (a machine number) | omitted (normal)
  pane    optional; defaults to the resolved HQ pane

  RED IS FOR DECISIONS. A machine number that is permanently red is a line nobody
  reads; colour is worth something because it is scarce.`,
		`gtmux brief — 把带色的简报写进中控 pane

  从 STDIN 读 JSON，渲染成带色、列对齐的文本，写入 pane 的 tty —— 那是人「看得见」
  的通道；唤醒线那条是打进输入框给 agent「读」的，不是这个。

  echo '{"grade":"decision","headline":"两个会话在等你拍板",
         "lines":[{"label":"api","value":"已等 11 分钟","tone":"decision"},
                  {"label":"磁盘","value":"剩 41GB","tone":"dim"}]}' | gtmux brief

  grade   decision (◆ 红) | attention (▸ 青) | ledger (· 暗)
  tone    逐行：decision (红) | dim (机器数字) | 不填 (普通)
  pane    可选，默认解析 HQ pane

  红色只留给需要你拍板的事。长期发红的机器数字等于没人看；颜色的价值来自稀缺。`
}
