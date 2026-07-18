package app

import (
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdStatus prints a compact, colored radar summary for the tmux status line
// (D5) — so the same waiting/working/idle language gtmux shows in the menu bar
// and phone is visible inside tmux itself. Add to ~/.tmux.conf:
//
//	set -g status-right '#(gtmux status)'
//
// It emits ONLY the non-zero per-state counts, each with the authoritative status
// color + shape (waiting=red ‖, working=cyan ●, idle=green ✓), using tmux
// `#[fg=...]` markup. Empty output when nothing is waiting/working/idle (the
// segment just disappears). `--plain` drops the tmux markup, for other status
// bars or testing.
func cmdStatus(args []string) int {
	plain := false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			i18n.Say("usage: gtmux status [--plain]   (tmux: set -g status-right '#(gtmux status)')",
				"用法：gtmux status [--plain]   （tmux：set -g status-right '#(gtmux status)'）")
			return 0
		case "--plain":
			plain = true
		default:
			i18n.Sae("gtmux status: unknown option '"+a+"'", "gtmux status: 未知选项 '"+a+"'")
			return 2
		}
	}
	if !tmux.ServerUp() {
		return 0 // nothing running → empty segment
	}
	if s := statusLine(radar.GatherAgents(), plain); s != "" {
		fmt.Print(s)
	}
	return 0
}

// statusLine renders the per-state counts as a tmux status segment. Pure (no I/O)
// so it's unit-testable. Order matches the radar's priority: waiting → working →
// idle; `running` (started but not demanding attention) is intentionally omitted
// to keep the segment glanceable. Colors are the authoritative status hex.
func statusLine(panes []radar.Pane, plain bool) string {
	var waiting, working, idle int
	for _, p := range panes {
		switch p.Status {
		case "waiting":
			waiting++
		case "working":
			working++
		case "idle":
			idle++
		}
	}
	segs := []struct {
		n     int
		glyph string
		color string
	}{
		{waiting, "‖", "#EF4444"},
		{working, "●", "#06B6D4"},
		{idle, "✓", "#22C55E"},
	}
	var parts []string
	for _, s := range segs {
		if s.n == 0 {
			continue
		}
		if plain {
			parts = append(parts, fmt.Sprintf("%s%d", s.glyph, s.n))
		} else {
			parts = append(parts, fmt.Sprintf("#[fg=%s]%s%d", s.color, s.glyph, s.n))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	out := strings.Join(parts, " ")
	if !plain {
		out += "#[default]"
	}
	return out
}
