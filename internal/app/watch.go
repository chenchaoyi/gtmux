package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

const watchInterval = 1500 * time.Millisecond

type tickMsg time.Time

var (
	stWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan
	stIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	stRun     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	stDimW    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	stBoldW   = lipgloss.NewStyle().Bold(true)
	stSel     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13")) // magenta
)

type watchModel struct {
	panes      []agentPane
	sel        int
	prev       map[string]string // paneID → last status (for transition detection)
	finished   map[string]bool   // panes that went working→idle during this session
	quitOnJump bool              // close the TUI after a jump (popup mode)
}

func runWatch(quitOnJump bool) int {
	p := gatherAgents()
	m := watchModel{panes: p, prev: statusMap(p), finished: map[string]bool{}, quitOnJump: quitOnJump}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		i18n.Sae("watch failed: "+err.Error(), "watch 失败："+err.Error())
		return 1
	}
	return 0
}

func statusMap(p []agentPane) map[string]string {
	m := make(map[string]string, len(p))
	for _, a := range p {
		m[a.paneID] = a.status
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
				id := m.panes[m.sel].paneID
				delete(m.finished, id) // acknowledged
				jumpCmd := func() tea.Msg { jumpPane(id); return nil }
				if m.quitOnJump {
					return m, tea.Sequence(jumpCmd, tea.Quit)
				}
				return m, jumpCmd
			}
		}
	case tickMsg:
		m.refresh()
		return m, tick()
	}
	return m, nil
}

func (m *watchModel) refresh() {
	p := gatherAgents()
	for _, a := range p {
		// Flag a row that just finished (working → idle). "waiting" has its own
		// prominent status, so don't double-flag it as "done".
		if m.prev[a.paneID] == "working" && a.status == "idle" {
			m.finished[a.paneID] = true
		}
		if a.status == "working" || a.status == "waiting" {
			delete(m.finished, a.paneID)
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

func (m watchModel) View() string {
	var b strings.Builder
	b.WriteString(stBoldW.Render("gtmux "+i18n.Tr("agents (live)", "agent（实时）")) +
		" — " + agentsSummary(m.panes) + "\n\n")

	if len(m.panes) == 0 {
		b.WriteString(stDimW.Render(i18n.Tr("No coding-agent panes found.", "没有发现 coding-agent 的 pane。")) + "\n")
	}
	for i, p := range m.panes {
		var st lipgloss.Style
		var glyph, label string
		switch p.status {
		case "working":
			st, glyph, label = stWorking, "⠿", i18n.Tr("working", "运行中")
		case "waiting":
			st, glyph, label = stRun, "⏸", i18n.Tr("waiting", "等输入")
		case "idle":
			st, glyph, label = stIdle, "✳", i18n.Tr("idle", "空闲")
		default:
			st, glyph, label = stRun, "●", i18n.Tr("running", "运行中")
		}
		prefix := "  "
		if i == m.sel {
			prefix = stSel.Render("❯ ")
		}
		task := p.task
		if task == "" {
			task = stDimW.Render("—")
		}
		tag := ""
		if p.latest || m.finished[p.paneID] {
			tag = stRun.Render(i18n.Tr("  ✓ done", "  ✓ 完成"))
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s %s%s%s\n",
			prefix,
			st.Render(glyph+" "+i18n.PadRight(label, 8)),
			stBoldW.Render(i18n.PadRight(p.agent, 12)),
			stBoldW.Render(i18n.PadRight(p.loc, 22)),
			task,
			stDimW.Render(" "+p.paneID),
			tag))
	}

	b.WriteString("\n" + stDimW.Render(i18n.Tr(
		"↑/↓ select · enter jump · r refresh · q quit",
		"↑/↓ 选择 · enter 跳转 · r 刷新 · q 退出")))
	return b.String()
}
