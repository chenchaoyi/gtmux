package radar

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// PaneRow is one tmux pane in the pane-browser enumeration (tiered-pane-control):
// EVERY pane, not just coding agents. `Tier` distinguishes an agent pane ("agent")
// from a plain shell/other ("plain") so a surface can badge agents and link to their
// Detail while still offering focus/type/attach on any pane. This is the superset of
// `gtmux agents` — the radar stays agent-only; the browser sees everything.
type PaneRow struct {
	PaneID  string `json:"pane_id"`
	Loc     string `json:"loc"` // session:window.pane
	Session string `json:"session"`
	Window  string `json:"window"` // window index
	Pane    string `json:"pane"`   // pane index
	// WinID / WinName are the window's STABLE tmux id and its (drifting) name —
	// tmux-id-surface. The id is the anchor and the name is a gloss, because a window's
	// name follows `automatic-rename` while `@N` does not move.
	//
	// The INDEX cannot play that role: measured on a real fleet of 13 sessions, 8 of 12
	// active windows had index 0, so grouping or labelling by index makes most windows
	// indistinguishable. `@N` is unique per server.
	//
	// NAMED `win_id`, deliberately NOT `window_id`-adjacent to `session_id`: the existing
	// `session_id` on an agent row is the coding-agent ADOPT key, nothing to do with
	// tmux's `$N`, and a symmetrical name here would invite exactly that confusion.
	//
	// Additive + omitempty: a surface built before this keeps decoding.
	WinID   string `json:"win_id,omitempty"`   // tmux window_id, "@N"
	WinName string `json:"win_name,omitempty"` // window_name (a gloss; may drift or repeat)
	Cwd     string `json:"cwd,omitempty"`
	Command string `json:"command"` // pane_current_command (bash / claude / vim …)
	Title   string `json:"title,omitempty"`
	Active  bool   `json:"active"`            // the active pane in its window
	InMode  bool   `json:"in_mode,omitempty"` // copy/view-mode → input is swallowed
	Tier    string `json:"tier"`              // "agent" | "plain"
	Agent   string `json:"agent,omitempty"`   // display name when Tier=="agent"
	Icon    string `json:"icon,omitempty"`    // identity icon hint (.app/image path) when Tier=="agent"
	// Git identity of the pane's cwd, on EVERY tier — the agent rows have carried it
	// since radar++, and a plain pane is exactly where someone does git by hand. A
	// surface reads Branch to know whether this pane HAS a repo at all: the phone shows
	// its Diff control only then, instead of offering a button that opens an empty sheet
	// (`GET /api/diff` returns "" for a non-repo cwd). gitInfo is filesystem-only — no
	// subprocess — so this stays cheap per pane per poll.
	Project string `json:"project,omitempty"` // repo-root basename ("" outside a repo)
	Branch  string `json:"branch,omitempty"`  // current branch or short SHA ("" outside a repo)
}

// panesSource lists every tmux pane with the fields the browser needs. A package var
// so fixture tests can inject panes without a live tmux server.
var panesSource = func() []string {
	// New fields go on the END: the parser reads by index, so appending cannot disturb
	// what is already there.
	const fields = "#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t" +
		"#{pane_current_path}\t#{pane_current_command}\t#{pane_title}\t#{pane_active}\t#{pane_in_mode}\t" +
		"#{window_id}\t#{window_name}"
	return tmux.Lines("list-panes", "-a", "-F", fields)
}

// agentPaneSet is the set of pane ids the radar classifies as coding agents (the
// full classification: title glyph + process subtree). A package var so tests can
// stub it without driving GatherAgents.
var agentPaneSet = func() (map[string]string, map[string]bool, map[string]string) {
	names := map[string]string{} // pane id → agent display name
	agents := map[string]bool{}
	icons := map[string]string{} // pane id → official-icon hint (drives the browser avatar)
	for _, p := range GatherAgents() {
		// A WATCHED row is a user-pinned PLAIN pane, NOT a coding agent — GatherAgents
		// appends it so the radar can show it, but it must stay tier="plain" in the
		// browser (else it's mislabeled an agent, tagged "on radar", and shows no $_
		// glyph). Only true agent rows set the agent tier.
		if p.source == "tmux" && !p.Watched {
			agents[p.PaneID] = true
			names[p.PaneID] = p.Agent
			icons[p.PaneID] = p.icon
		}
	}
	return names, agents, icons
}

// GatherPanes enumerates every tmux pane, tagging each with its tier by
// cross-referencing the agent radar — so "agent" here means exactly what the radar
// means by it (no duplicated classification), and everything else is "plain".
func GatherPanes() []PaneRow {
	names, agents, icons := agentPaneSet()
	var out []PaneRow
	for _, line := range panesSource() {
		f := strings.SplitN(line, "\t", 11)
		if len(f) < 6 {
			continue
		}
		id := f[0]
		tier := "plain"
		if agents[id] {
			tier = "agent"
		}
		row := PaneRow{
			PaneID:  id,
			Session: f[1],
			Window:  f[2],
			Pane:    f[3],
			Loc:     f[1] + ":" + f[2] + "." + f[3],
			Cwd:     f[4],
			Command: f[5],
			Tier:    tier,
			Agent:   names[id],
			Icon:    icons[id],
		}
		row.Project, row.Branch = gitInfo(row.Cwd)
		if len(f) >= 7 {
			row.Title = meaningfulTitle(f[6])
		}
		if len(f) >= 8 {
			row.Active = f[7] == "1"
		}
		if len(f) >= 9 {
			row.InMode = f[8] == "1"
		}
		if len(f) >= 10 {
			row.WinID = strings.TrimSpace(f[9])
		}
		if len(f) >= 11 {
			row.WinName = strings.TrimSpace(f[10])
		}
		out = append(out, row)
	}
	return out
}

// hostTitle is this machine's hostname, which a shell commonly writes into every pane's
// title. Resolved once — it cannot change while the process runs.
var hostTitle = func() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(h)
}()

// meaningfulTitle drops a pane title that does not NAME the pane.
//
// A shell writes the hostname into the title of every pane it owns, so on a real fleet
// four different panes all read `ccy-MBP2024-M4-Office.local` — a label that tells them
// apart from nothing. (A sibling shell with no title showed `bash`, which is at least
// true.) The tmux-id-surface design says it plainly: `pane_title` is not a usable
// per-pane name, so a surface must fall back to the command.
//
// Dropped HERE, in the core, rather than in each surface: only this machine knows its own
// hostname — the phone and the web are clients and cannot check it. Same reason the HQ
// verdict is decided once in the core and merely rendered by the surfaces.
func meaningfulTitle(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return ""
	}
	if hostTitle != "" && (t == hostTitle || t == strings.SplitN(hostTitle, ".", 2)[0]) {
		return ""
	}
	return t
}

// PanesJSONBytes marshals GatherPanes for `gtmux panes --json` / any HTTP consumer.
// Always a JSON array (never null) so consumers can decode unconditionally.
func PanesJSONBytes() ([]byte, error) {
	rows := GatherPanes()
	if rows == nil {
		rows = []PaneRow{}
	}
	return json.Marshal(rows)
}
