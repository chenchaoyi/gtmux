// Package menubar holds the cgo-free model behind the gtmux menu-bar app: the
// agents --json schema it consumes, and the pure functions that turn that data
// into a status-bar title, a summary line, and dropdown rows. Keeping this
// separate from the cgo systray glue (cmd/gtmux-menubar) makes the logic
// testable on any platform and keeps the contract in one place.
package menubar

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
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

// Title is the compact status-bar text (glyph + count). Paired with IconFor's
// colored dot it's somewhat redundant, so the app uses BadgeText for the title;
// Title remains a self-contained text representation (e.g. for tooltips).
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

// BadgeText is the menu-bar title shown next to the colored icon: the count of
// the most-urgent actionable state, or "" when nothing needs attention.
func BadgeText(agents []Agent) string {
	w, wk := counts(agents)
	switch {
	case w > 0:
		return strconv.Itoa(w)
	case wk > 0:
		return strconv.Itoa(wk)
	default:
		return ""
	}
}

// Summary mirrors the CLI's "N agents · X waiting · Y working · Z idle"
// (localized via GTMUX_LANG, like the CLI's agents summary).
func Summary(agents []Agent) string {
	n := len(agents)
	if n == 0 {
		return i18n.Tr("no agents", "没有 agent")
	}
	w, wk := counts(agents)
	parts := make([]string, 0, 3)
	if w > 0 {
		parts = append(parts, fmt.Sprintf(i18n.Tr("%d waiting", "%d 等输入"), w))
	}
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d working", "%d 运行中"), wk))
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d idle", "%d 空闲"), n-w-wk))
	return i18n.Pl(n, "agent") + " · " + strings.Join(parts, " · ")
}

// FilterWaiting returns only the agents blocked on the user.
func FilterWaiting(agents []Agent) []Agent {
	out := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if a.Status == "waiting" {
			out = append(out, a)
		}
	}
	return out
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

// truncate shortens s to at most max display runes, appending an ellipsis.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimRight(string(r[:max-1]), " ") + "…"
}
