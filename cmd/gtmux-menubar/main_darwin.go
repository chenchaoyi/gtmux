//go:build darwin

// Command gtmux-menubar is the macOS menu-bar form of gtmux: a persistent
// LSUIElement status item showing live coding-agent status (waiting/working/
// idle) with click-to-jump. It is a CONSUMER of the gtmux CLI — it shells out to
// `gtmux agents --json` to read state and `gtmux focus <pane>` to jump — so the
// CLI stays the single agent-agnostic data source. This binary needs cgo
// (Cocoa via fyne.io/systray); the CLI binary stays cgo-free.
package main

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"

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
		go watchClick(i, rows[i])
	}

	systray.AddSeparator()
	waitingItem := systray.AddMenuItemCheckbox(i18n.Tr("Waiting only", "仅看等输入"), "", false)
	refreshNow := systray.AddMenuItem(i18n.Tr("Refresh", "刷新"), "")
	quit := systray.AddMenuItem(i18n.Tr("Quit", "退出"), "")

	// wake coalesces every "refresh now" trigger (toggle, manual, fs event).
	wake := make(chan struct{}, 1)
	poke := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}

	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()
	go func() {
		for range waitingItem.ClickedCh {
			if waitingItem.Checked() {
				waitingItem.Uncheck()
				filterWaiting.Store(false)
			} else {
				waitingItem.Check()
				filterWaiting.Store(true)
			}
			poke()
		}
	}()
	go func() {
		for range refreshNow.ClickedCh {
			poke()
		}
	}()

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

// watchClick jumps to the pane currently shown in row i when it's clicked.
func watchClick(i int, item *systray.MenuItem) {
	for range item.ClickedCh {
		mu.Lock()
		id := panes[i]
		mu.Unlock()
		if id != "" {
			_ = exec.Command(gtmuxBin, "focus", id).Run()
		}
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
