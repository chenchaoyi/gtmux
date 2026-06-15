//go:build darwin

// Command gtmux-menubar is the macOS menu-bar form of gtmux: a persistent
// LSUIElement status item showing live coding-agent status (waiting/working/
// idle) with click-to-jump. It is a CONSUMER of the gtmux CLI — it shells out to
// `gtmux agents --json` to read state and `gtmux focus <pane>` to jump — so the
// CLI stays the single agent-agnostic data source. This binary needs cgo
// (Cocoa via fyne.io/systray); the CLI binary stays cgo-free.
package main

import (
	"os/exec"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/chenchaoyi/gtmux/internal/menubar"
)

const (
	pollInterval = 1500 * time.Millisecond
	// systray can't add/remove items after start, so we pre-allocate a fixed
	// pool of agent rows and show/hide them per refresh. 24 is comfortably more
	// than anyone runs at once.
	maxRows = 24
)

var (
	gtmuxBin string

	mu    sync.Mutex
	panes []string // row index → current pane id ("" when the row is unused)
)

func main() {
	gtmuxBin = menubar.ResolveGtmux()
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetTitle("✳")
	systray.SetTooltip("gtmux — agents")

	header := systray.AddMenuItem("no agents", "")
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
	refreshNow := systray.AddMenuItem("Refresh", "Poll gtmux now")
	quit := systray.AddMenuItem("Quit", "Quit the gtmux menu bar")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()

	// Poll loop: refresh on a timer, or immediately when "Refresh" is clicked.
	go func() {
		for {
			refresh(header, rows)
			select {
			case <-time.After(pollInterval):
			case <-refreshNow.ClickedCh:
			}
		}
	}()
}

// refresh re-reads agents and updates the title, tooltip, header, and rows.
func refresh(header *systray.MenuItem, rows []*systray.MenuItem) {
	agents := fetch()
	systray.SetTitle(menubar.Title(agents))
	summary := menubar.Summary(agents)
	systray.SetTooltip("gtmux — " + summary)
	header.SetTitle(summary)

	rendered := menubar.Rows(agents)
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
