package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/panefocus"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

const watchInterval = 1500 * time.Millisecond

type tickMsg time.Time

// The status language, in the terminal, from DESIGN §9's own values — lipgloss maps a
// hex down to the nearest colour the terminal actually has, so a 16-colour terminal still
// gets red for waiting rather than the yellow it used to get.
//
// Waiting was drawn with stRun for years, which is the same yellow as running: the one
// state that wants you looked exactly like the one that wants nothing. Every other
// surface obeyed the rule; this one did not.
var (
	stWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	stWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	stIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	stRun     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8E8E93"))
	stAmber   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	stDimW    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stBoldW   = lipgloss.NewStyle().Bold(true)
	stSel     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C4B5FD"))
)

// watchStyle is the row's style, glyph and label for a status. It reads the SAME glyph
// table the one-shot `gtmux agents` reads, so the two screens cannot drift.
func watchStyle(status string) (lipgloss.Style, string, string) {
	switch status {
	case "working":
		return stWorking, agentGlyph[status], i18n.Tr("working", "运行中")
	case "waiting":
		return stWaiting, agentGlyph[status], i18n.Tr("waiting", "等输入")
	case "idle":
		return stIdle, agentGlyph[status], i18n.Tr("idle", "空闲")
	default:
		return stRun, agentGlyph["running"], i18n.Tr("running", "运行中")
	}
}

// sectionOf is the heading a status sits under, in DESIGN's own order: needs you first,
// a bare shell last. "" means the row gets no heading of its own.
func sectionOf(status string) string {
	switch status {
	case "waiting":
		return i18n.Tr("NEEDS YOU", "等你")
	case "working":
		return i18n.Tr("WORKING", "运行中")
	case "idle":
		return i18n.Tr("IDLE", "空闲")
	default:
		return i18n.Tr("RUNNING", "只有 shell")
	}
}

// sinceShort is the age column. humanize.AgeShort says "just now" under a minute, which
// is the right answer for a board read once an hour and the wrong one for a screen that
// redraws every 1.5 seconds: "40s" is the number a person is actually watching.
func sinceShort(secs int64) string {
	switch {
	case secs <= 0:
		return ""
	case secs < 60:
		return strconv.FormatInt(secs, 10) + "s"
	case secs < 3600:
		return strconv.FormatInt(secs/60, 10) + "m"
	case secs < 48*3600:
		return strconv.FormatInt(secs/3600, 10) + "h"
	default:
		return strconv.FormatInt(secs/(24*3600), 10) + "d"
	}
}

type watchModel struct {
	panes      []radar.Pane
	width      int // the terminal's columns; 0 until the first WindowSizeMsg
	sel        int
	prev       map[string]string // paneID → last status (for transition detection)
	finished   map[string]bool   // panes that went working→idle during this session
	quitOnJump bool              // close the TUI after a jump (popup mode)
}

func runWatch(quitOnJump bool) int {
	p := radar.GatherAgents()
	m := watchModel{panes: p, prev: statusMap(p), finished: map[string]bool{}, quitOnJump: quitOnJump}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		i18n.Sae("watch failed: "+err.Error(), "watch 失败："+err.Error())
		return 1
	}
	return 0
}

func statusMap(p []radar.Pane) map[string]string {
	m := make(map[string]string, len(p))
	for _, a := range p {
		m[a.PaneID] = a.Status
	}
	return m
}

func tick() tea.Cmd {
	return tea.Tick(watchInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m watchModel) Init() tea.Cmd { return tick() }

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
		case "down", "j":
			if m.sel < len(m.panes)-1 {
				m.sel++
			}
		case "r":
			m.refresh()
		case "enter":
			if m.sel >= 0 && m.sel < len(m.panes) {
				id := m.panes[m.sel].PaneID
				delete(m.finished, id) // acknowledged
				jumpCmd := func() tea.Msg { panefocus.JumpPane(id); return nil }
				if m.quitOnJump {
					return m, tea.Sequence(jumpCmd, tea.Quit)
				}
				return m, jumpCmd
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tickMsg:
		m.refresh()
		return m, tick()
	}
	return m, nil
}

func (m *watchModel) refresh() {
	p := radar.GatherAgents()
	for _, a := range p {
		// Flag a row that just finished (working → idle). "waiting" has its own
		// prominent status, so don't double-flag it as "done".
		if m.prev[a.PaneID] == "working" && a.Status == "idle" {
			m.finished[a.PaneID] = true
		}
		if a.Status == "working" || a.Status == "waiting" {
			delete(m.finished, a.PaneID)
		}
	}
	m.prev = statusMap(p)
	m.panes = p
	if m.sel >= len(p) {
		m.sel = len(p) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

// View draws the live board. It is GROUPED, the way every other surface groups it — the
// flat list gave a reader nothing to aim at, and the counts at the top were the only
// thing saying how much of it needed them.
//
// Widths adapt because a terminal is not 96 columns by decree: the agent column is the
// first to go, then the task shrinks. Nothing wraps; a wrapped row in a screen that
// redraws every 1.5s reads as corruption.
func (m watchModel) View() string {
	width := m.width
	if width <= 0 {
		width = 96
	}
	var b strings.Builder

	head := "gtmux " + i18n.Tr("agents (live)", "agent（实时）")
	summary := agentsSummary(m.panes)
	if room := width - i18n.DispWidth(head) - 2; i18n.DispWidth(summary) > room {
		summary = i18n.TruncDisp(summary, maxInt(0, room))
	}
	b.WriteString(stBoldW.Render(head))
	b.WriteString(i18n.PadRight("", maxInt(1, width-i18n.DispWidth(head)-i18n.DispWidth(summary))))
	b.WriteString(stDimW.Render(summary) + "\n")

	if len(m.panes) == 0 {
		b.WriteString("\n" + stDimW.Render(i18n.Tr("No coding-agent panes found.", "没有发现 coding-agent 的 pane。")) + "\n")
		return b.String()
	}

	// Column plan. agentW 0 drops the column: the glyph and the location already say who
	// and where, and the task is what a reader came for.
	agentW, locW, taskW := 13, 13, 34
	if width < 96 {
		agentW = 0
		locW, taskW = 13, maxInt(12, width-2-10-13-8-12)
	}
	if width >= 96 {
		taskW = maxInt(12, width-2-10-agentW-locW-8-12)
	}

	tailHdr := i18n.PadLeft(i18n.Tr("pane", "pane"), 5) + i18n.PadLeft(i18n.Tr("since", "有多久"), 7)
	b.WriteString(i18n.PadRight("", maxInt(1, width-i18n.DispWidth(tailHdr))))
	b.WriteString(stDimW.Render(tailHdr) + "\n")

	section := ""
	for i, p := range m.panes {
		if sec := sectionOf(p.Status); sec != section {
			section = sec
			n := 0
			for _, q := range m.panes {
				if sectionOf(q.Status) == sec {
					n++
				}
			}
			head := stDimW.Render(sec + "  " + fmt.Sprint(n))
			if p.Status == "waiting" {
				head = stWaiting.Render(sec) + stDimW.Render("  "+fmt.Sprint(n))
			}
			b.WriteString(head + "\n")
		}
		st, glyph, label := watchStyle(p.Status)
		prefix := "  "
		if i == m.sel {
			prefix = stSel.Render("❯ ")
		}
		task := p.Task
		if task == "" {
			task = "—"
		}
		task = i18n.TruncDisp(task, taskW)
		tag := ""
		if p.Latest || m.finished[p.PaneID] {
			tag = i18n.Tr("latest", "最近完成")
		}
		row := prefix + st.Render(glyph+" "+i18n.PadRight(label, 8))
		if agentW > 0 {
			row += stBoldW.Render(i18n.PadRight(p.Agent, agentW))
		}
		row += stBoldW.Render(i18n.PadRight(p.Loc, locW))
		if p.Task == "" {
			row += stDimW.Render(i18n.PadRight(task, taskW))
		} else {
			row += i18n.PadRight(task, taskW)
		}
		row += stIdle.Render(i18n.PadRight(tag, 8))
		tail := stDimW.Render(i18n.PadLeft(p.PaneID, 5)) + stDimW.Render(i18n.PadLeft(sinceShort(time.Now().Unix()-p.Since), 7))
		used := 2 + 10 + agentW + locW + taskW + 8 + 12
		if gap := width - used; gap > 0 {
			row += i18n.PadRight("", gap)
		}
		b.WriteString(row + tail + "\n")
		if i+1 < len(m.panes) && sectionOf(m.panes[i+1].Status) != section {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n" + stDimW.Render(strings.Repeat("─", width)) + "\n")
	keys := i18n.Tr("↑↓ select · ⏎ jump · r refresh · q quit", "↑↓ 选择 · ⏎ 跳转 · r 刷新 · q 退出")
	plain, styled := quotaStrip()
	if plain == "" {
		b.WriteString(i18n.PadRight("", maxInt(1, width-i18n.DispWidth(keys))) + stDimW.Render(keys))
		return b.String()
	}
	b.WriteString(styled)
	b.WriteString(i18n.PadRight("", maxInt(1, width-i18n.DispWidth(plain)-i18n.DispWidth(keys))))
	b.WriteString(stDimW.Render(keys))
	return b.String()
}

// quotaStrip is the one line of plan state the board carries, and it is here because this
// is the screen people leave open. Tightest window first, the full one in amber, and a
// window using nothing is not news — codex at 0% would spend a quarter of the line saying
// so. Cache only: this redraws every 1.5s and must never spawn an agent to ask.
func quotaStrip() (plain, styled string) {
	rep, ok := limits.Get(limits.LoadConfig(), false, time.Now())
	if !ok || len(rep.Windows) == 0 {
		return "", ""
	}
	wins := append([]limits.Window(nil), rep.Windows...)
	sort.SliceStable(wins, func(i, j int) bool { return wins[i].PctUsed > wins[j].PctUsed })
	parts := make([]string, 0, 3)
	styledParts := make([]string, 0, 3)
	for _, w := range wins {
		if w.PctUsed <= 0 || len(parts) == 3 {
			continue
		}
		txt := fmt.Sprintf("%s %d%%", limits.Name(w), w.PctUsed)
		parts = append(parts, txt)
		if w.Tier == limits.TierWarn || w.Tier == limits.TierFull {
			styledParts = append(styledParts, stAmber.Render(txt))
		} else {
			styledParts = append(styledParts, txt)
		}
	}
	if len(parts) == 0 {
		return "", ""
	}
	lead := i18n.Tr("plan  ", "额度  ")
	return lead + strings.Join(parts, " · "), stDimW.Render(lead) + strings.Join(styledParts, stDimW.Render(" · "))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
