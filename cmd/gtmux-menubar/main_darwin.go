//go:build darwin

// Command gtmux-menubar is the macOS menu-bar form of gtmux: a persistent
// LSUIElement status item showing live coding-agent status (waiting/working/
// idle) with click-to-jump. It is a CONSUMER of the gtmux CLI — it shells out to
// `gtmux agents --json` to read state and `gtmux focus <pane>` to jump — so the
// CLI stays the single agent-agnostic data source. This binary needs cgo
// (Cocoa); the CLI binary stays cgo-free.
//
// Rendering uses github.com/energye/systray, a maintained fyne.io/systray fork
// that creates a visible NSStatusItem on macOS 26 ("Tahoe"); upstream
// fyne.io/systray v1.12.2 does not render on 26 (the process runs but the bar is
// empty — see Phase-3.1). Only the rendering layer differs; the contracts hold.
package main

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/menubar"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	pollInterval  = 1500 * time.Millisecond
	watchDebounce = 200 * time.Millisecond
	// systray can't add/remove items after start, so we pre-allocate a fixed
	// pool of agent rows and show/hide them per refresh. 24 is comfortably more
	// than anyone runs at once.
	maxRows = 24
)

var (
	gtmuxBin string

	mu    sync.Mutex
	panes []string // row index → current pane id ("" when the row is unused)

	filterWaiting atomic.Bool // "Waiting only" menu toggle
)

func main() {
	i18n.SetLang(os.Getenv("GTMUX_LANG")) // chrome localization; default en
	gtmuxBin = menubar.ResolveGtmux()
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetIcon(menubar.IconFor(nil))
	systray.SetTitle("")
	systray.SetTooltip("gtmux")

	header := systray.AddMenuItem("", "")
	header.Disable()
	systray.AddSeparator()

	rows := make([]*systray.MenuItem, maxRows)
	panes = make([]string, maxRows)
	for i := range rows {
		rows[i] = systray.AddMenuItem("", "")
		rows[i].Hide()
		idx := i
		rows[i].Click(func() { jumpRow(idx) })
	}

	systray.AddSeparator()
	waitingItem := systray.AddMenuItemCheckbox(i18n.Tr("Waiting only", "仅看等输入"), "", false)
	refreshNow := systray.AddMenuItem(i18n.Tr("Refresh", "刷新"), "")

	systray.AddSeparator()
	overviewItem := systray.AddMenuItem(i18n.Tr("Overview", "概览"), i18n.Tr("Sessions/windows/panes in a window", "在新窗口看 session/window/pane"))
	watchItem := systray.AddMenuItem(i18n.Tr("Live watch…", "实时面板…"), i18n.Tr("Open the full agents dashboard", "打开完整 agents 面板"))
	restoreItem := systray.AddMenuItem(i18n.Tr("Restore detached", "接回 detached"), i18n.Tr("Open a tab per detached session", "为每个 detached session 开 tab"))
	newItem := systray.AddMenuItem(i18n.Tr("New session", "新建 session"), i18n.Tr("Create a tmux session + tab", "新建 tmux session + tab"))

	systray.AddSeparator()
	quit := systray.AddMenuItem(i18n.Tr("Quit", "退出"), "")

	// wake coalesces every "refresh now" trigger (toggle, manual, fs event).
	wake := make(chan struct{}, 1)
	poke := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}

	quit.Click(func() { systray.Quit() })
	refreshNow.Click(poke)
	waitingItem.Click(func() {
		if waitingItem.Checked() {
			waitingItem.Uncheck()
			filterWaiting.Store(false)
		} else {
			waitingItem.Check()
			filterWaiting.Store(true)
		}
		poke()
	})

	// Menu actions — each shells out to the CLI / drives Ghostty (consumer only).
	overviewItem.Click(func() { _, _ = ghostty.OpenWindow(ghostty.ShellQuote(gtmuxBin) + " overview --hold") })
	watchItem.Click(func() { _, _ = ghostty.OpenWindow(ghostty.ShellQuote(gtmuxBin) + " agents --watch") })
	restoreItem.Click(func() { _ = exec.Command(gtmuxBin, "restore").Start() })
	newItem.Click(func() { _ = exec.Command(gtmuxBin, "new").Start() })

	// Hybrid updates: fsnotify makes waiting/active changes instant; the timer
	// still catches working/idle, which come from pane titles (not state files).
	if events, _, err := menubar.WatchState(state.Dir(), watchDebounce); err == nil {
		go func() {
			for range events {
				poke()
			}
		}()
	}

	go func() {
		for {
			refresh(header, rows)
			select {
			case <-time.After(pollInterval):
			case <-wake:
			}
		}
	}()
}

// refresh re-reads agents and updates the icon, badge, tooltip, header, and rows.
func refresh(header *systray.MenuItem, rows []*systray.MenuItem) {
	agents := fetch()
	systray.SetIcon(menubar.IconFor(agents))
	systray.SetTitle(menubar.BadgeText(agents))
	summary := menubar.Summary(agents)
	systray.SetTooltip("gtmux — " + summary)

	shown := agents
	if filterWaiting.Load() {
		shown = menubar.FilterWaiting(agents)
		summary += i18n.Tr(" · waiting only", " · 仅看等输入")
	}
	header.SetTitle(summary)

	rendered := menubar.Rows(shown)
	mu.Lock()
	defer mu.Unlock()
	for i, item := range rows {
		if i < len(rendered) {
			item.SetTitle(rendered[i].Label)
			item.SetTooltip(rendered[i].Tooltip)
			panes[i] = rendered[i].PaneID
			item.Show()
		} else {
			panes[i] = ""
			item.Hide()
		}
	}
}

// jumpRow runs `gtmux focus` for the pane currently shown in row i.
func jumpRow(i int) {
	mu.Lock()
	id := panes[i]
	mu.Unlock()
	if id != "" {
		_ = exec.Command(gtmuxBin, "focus", id).Run()
	}
}

// fetch runs `gtmux agents --json`; any error (no server, missing binary) yields
// an empty list so the menu shows "no agents" rather than crashing.
func fetch() []menubar.Agent {
	out, err := exec.Command(gtmuxBin, "agents", "--json").Output()
	if err != nil {
		return nil
	}
	agents, _ := menubar.Parse(out)
	return agents
}
