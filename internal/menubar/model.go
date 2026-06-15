// Package menubar holds the cgo-free model behind the gtmux menu-bar app: the
// agents --json schema it consumes, and the pure functions that turn that data
// into a status-bar title, a summary line, and dropdown rows. Keeping this
// separate from the cgo systray glue (cmd/gtmux-menubar) makes the logic
// testable on any platform and keeps the contract in one place.
package menubar

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Agent mirrors one element of `gtmux agents --json` (the stable contract from
// internal/app). The menu-bar app is a consumer of that published shape, so this
// intentionally tracks the JSON, not gtmux's internal struct.
type Agent struct {
	PaneID   string `json:"pane_id"` // %N — the jump target: gtmux focus <pane_id>
	Session  string `json:"session"`
	Window   string `json:"window"`
	Pane     string `json:"pane"`
	Loc      string `json:"loc"`
	Agent    string `json:"agent"`
	Status   string `json:"status"` // working | waiting | idle | running
	Task     string `json:"task"`
	Latest   bool   `json:"latest"`
	Activity bool   `json:"activity"`
}

// Row is one rendered dropdown entry.
type Row struct {
	Label   string // "‹glyph› session · task"
	Tooltip string // "loc · agent · status"
	PaneID  string // jump target ("" → not clickable)
}

// Parse decodes `gtmux agents --json` output. Empty/whitespace input (e.g. no
// tmux server) yields no agents, not an error.
func Parse(b []byte) ([]Agent, error) {
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, nil
	}
	var agents []Agent
	if err := json.Unmarshal(b, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// counts returns how many agents are waiting / working (running and idle fall
// outside both — they're the calm states).
func counts(agents []Agent) (waiting, working int) {
	for _, a := range agents {
		switch a.Status {
		case "waiting":
			waiting++
		case "working":
			working++
		}
	}
	return
}

// Title is the compact status-bar text. It reflects the most-urgent state with a
// count for the actionable ones; all-idle/empty shows a calm marker.
//
//	⏸2  → 2 agents waiting on you (most urgent)
//	⠿3  → nothing waiting, 3 working
//	✳   → nothing waiting or working
func Title(agents []Agent) string {
	w, wk := counts(agents)
	switch {
	case w > 0:
		return fmt.Sprintf("⏸%d", w)
	case wk > 0:
		return fmt.Sprintf("⠿%d", wk)
	default:
		return "✳"
	}
}

// Summary mirrors the CLI's "N agents · X waiting · Y working · Z idle".
func Summary(agents []Agent) string {
	n := len(agents)
	if n == 0 {
		return "no agents"
	}
	w, wk := counts(agents)
	parts := make([]string, 0, 3)
	if w > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", w))
	}
	parts = append(parts, fmt.Sprintf("%d working", wk))
	parts = append(parts, fmt.Sprintf("%d idle", n-w-wk))
	return fmt.Sprintf("%s · %s", plural(n, "agent"), strings.Join(parts, " · "))
}

// Rows renders one dropdown entry per agent, preserving the input order (the CLI
// already sorts waiting → working → idle, then by location).
func Rows(agents []Agent) []Row {
	rows := make([]Row, 0, len(agents))
	for _, a := range agents {
		label := a.Session
		if label == "" {
			label = a.Loc
		}
		detail := a.Task
		if detail == "" {
			detail = a.Agent
		}
		if detail != "" {
			label += " · " + truncate(detail, 48)
		}
		rows = append(rows, Row{
			Label:   StatusGlyph(a.Status) + " " + label,
			Tooltip: strings.TrimSpace(a.Loc + " · " + a.Agent + " · " + a.Status),
			PaneID:  a.PaneID,
		})
	}
	return rows
}

// StatusGlyph maps a status to its menu glyph (matching the CLI's palette).
func StatusGlyph(status string) string {
	switch status {
	case "working":
		return "⠿"
	case "waiting":
		return "⏸"
	case "idle":
		return "✳"
	default: // running
		return "●"
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// truncate shortens s to at most max display runes, appending an ellipsis.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimRight(string(r[:max-1]), " ") + "…"
}
