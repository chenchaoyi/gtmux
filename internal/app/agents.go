package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Claude Code's foreground process reports its command as its version (e.g.
// "2.1.177"), which is how we identify a Claude pane that is actively working
// (its title is "<spinner> <task>" then, with no "Claude Code" text).
var claudeVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// agentProfile identifies a coding agent and (optionally) its idle marker.
// Working state is detected generically from a braille-spinner title glyph
// (most agent TUIs animate one), so profiles mainly map command/name → label.
type agentProfile struct {
	Name      string   `json:"name"`                // display label, e.g. "Claude Code"
	Commands  []string `json:"commands"`            // pane_current_command matches
	IdleGlyph string   `json:"idleGlyph,omitempty"` // leading rune meaning idle (e.g. "✳")
	// Icon is an optional identity image the menu-bar app renders in the avatar
	// (DESIGN §6). It's a hint the app resolves: a ".app" path → that app's real
	// icon (no third-party logo is committed — it comes from the user's installed
	// app), or an image file path. Empty → the app's neutral monogram.
	Icon string `json:"icon,omitempty"`
}

// Built-in profiles. Extend or override via ~/.config/gtmux/agents.json
// (a JSON array of {name, commands, idleGlyph, icon}); user entries take
// precedence. Default icons point at the vendor's installed desktop app, so the
// avatar shows the real official logo when that app is present — without
// bundling any trademark.
var builtinProfiles = []agentProfile{
	{Name: "Claude Code", Commands: []string{"claude"}, IdleGlyph: "✳", Icon: "/Applications/Claude.app"},
	{Name: "Codex", Commands: []string{"codex"}, Icon: "/Applications/Codex.app"},
	{Name: "Gemini", Commands: []string{"gemini"}},
	{Name: "Aider", Commands: []string{"aider"}},
	{Name: "opencode", Commands: []string{"opencode"}},
	{Name: "Crush", Commands: []string{"crush"}},
	{Name: "Cursor", Commands: []string{"cursor-agent", "cursor"}, Icon: "/Applications/Cursor.app"},
	{Name: "Amp", Commands: []string{"amp"}},
}

func loadProfiles() []agentProfile {
	profiles := builtinProfiles
	path := os.Getenv("HOME") + "/.config/gtmux/agents.json"
	if b, err := os.ReadFile(path); err == nil {
		var user []agentProfile
		if json.Unmarshal(b, &user) == nil && len(user) > 0 {
			profiles = append(user, profiles...) // user entries win
		}
	}
	return profiles
}

// fileMtime returns a file's modtime in unix seconds, 0 if it doesn't exist.
func fileMtime(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime().Unix()
	}
	return 0
}

// iconFor returns the icon hint for the agent named name (first matching profile,
// so user overrides win). "" when none is configured.
func iconFor(name string, profiles []agentProfile) string {
	for i := range profiles {
		if profiles[i].Name == name {
			return profiles[i].Icon
		}
	}
	return ""
}

type agentPane struct {
	paneID   string
	session  string
	window   string // window index
	pane     string // pane index
	loc      string // session:window.pane
	agent    string // display name, "" if unknown type
	task     string // title with the status glyph stripped
	status   string // "working" | "waiting" | "idle" | "running"
	activity bool
	latest   bool // the most-recently-finished pane (claude-notify last-finished)
	// terminal generalization (DESIGN §7)
	source     string // "tmux" | "native"
	project    string
	terminal   string
	tab        string
	activityAt int64  // epoch seconds of last activity (relative time)
	since      int64  // epoch seconds the current state began (for a duration)
	icon       string // identity icon hint (.app path or image path); "" = monogram
}

// agentJSON is the stable shape emitted by `gtmux agents --json` (for scripts
// and the future menu-bar app — structured, no screen-scraping).
type agentJSON struct {
	PaneID   string `json:"pane_id"` // %N — jump target: gtmux focus <pane_id>
	Session  string `json:"session"`
	Window   string `json:"window"`
	Pane     string `json:"pane"`
	Loc      string `json:"loc"`
	Agent    string `json:"agent"`
	Status   string `json:"status"` // working | waiting | idle | running
	Task     string `json:"task"`
	Latest   bool   `json:"latest"`
	Activity bool   `json:"activity"`
	// terminal generalization (DESIGN §7): tmux agents carry session/window/pane;
	// native agents (run directly in a terminal) carry project/terminal/tab.
	Source     string `json:"source"`             // "tmux" | "native"
	Project    string `json:"project,omitempty"`  // native: cwd basename
	Terminal   string `json:"terminal,omitempty"` // native: terminal app
	Tab        string `json:"tab,omitempty"`      // native: terminal tab title (jump key)
	ActivityAt int64  `json:"activity_at,omitempty"`
	Since      int64  `json:"since,omitempty"` // epoch the current state began (duration)
	Icon       string `json:"icon,omitempty"`  // identity icon hint (.app/image path)
}

// isBrailleSpinner reports whether r is in the braille block (U+2800–U+28FF),
// the de-facto spinner glyph most agent TUIs animate while working.
func isBrailleSpinner(r rune) bool { return r >= 0x2800 && r <= 0x28FF }

// cmdIsLiveAgent reports whether the foreground command is a live agent process
// and its display name. Claude Code's foreground command is its version string.
func cmdIsLiveAgent(cmd string, profiles []agentProfile) (name string, live bool) {
	for _, p := range profiles {
		for _, c := range p.Commands {
			if cmd == c {
				return p.Name, true
			}
		}
	}
	if claudeVersionRe.MatchString(cmd) {
		return "Claude Code", true
	}
	return "", false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// classifyAgent decides whether a pane runs a LIVE coding agent, which one, and
// its status. A pane counts ONLY if the agent process is actually running (its
// foreground command is the agent) OR its title is animating a braille spinner
// (active work, e.g. a tool subprocess). A leftover agent title on a pane that
// has returned to a plain shell — e.g. resurrect-restored with the agent not
// relaunched, or the agent simply exited — does NOT count. That stale-title case
// was the false positive (a "✳ Claude Code" title over a bash prompt).
func classifyAgent(title, cmd string, profiles []agentProfile) (isAgent bool, agent, status, task string) {
	t := strings.TrimSpace(title)
	rs := []rune(t)

	spinner := len(rs) > 0 && isBrailleSpinner(rs[0])
	idle := false
	if len(rs) > 0 && !spinner {
		if rs[0] == 0x2733 { // ✳ → idle/ready (Claude Code's marker)
			idle = true
		} else {
			for _, p := range profiles {
				if p.IdleGlyph != "" && strings.HasPrefix(t, p.IdleGlyph) {
					idle = true
					break
				}
			}
		}
	}

	name, live := cmdIsLiveAgent(cmd, profiles)
	titleName := ""
	for i := range profiles {
		if strings.Contains(t, profiles[i].Name) {
			titleName = profiles[i].Name
			break
		}
	}

	switch {
	case spinner: // actively working — count regardless of foreground command
		status = "working"
		agent = firstNonEmpty(name, titleName, i18n.Tr("agent", "agent"))
	case live: // agent process is alive (idle at its prompt, or just running)
		if idle {
			status = "idle"
		} else {
			status = "running"
		}
		agent = firstNonEmpty(name, titleName, i18n.Tr("agent", "agent"))
	default: // plain shell with a leftover agent title → not actually running
		return false, "", "", ""
	}

	task = t
	if (spinner || idle) && len(rs) > 1 {
		task = strings.TrimSpace(string(rs[1:]))
	}
	if task == agent { // title is just the placeholder name, not a real task
		task = ""
	}
	return true, agent, status, task
}

// procInfo is one process row: its parent pid, cumulative CPU seconds, and full
// command line (argv).
type procInfo struct {
	ppid    int
	cpu     float64 // cumulative CPU seconds (for the hook-free working signal)
	command string
}

// snapshotProcs returns pid → {ppid, cpu, argv} for every process (one `ps`
// call), so we can look inside a pane's process tree and sum its CPU. Empty on
// any failure.
func snapshotProcs() map[int]procInfo {
	out := map[int]procInfo{}
	b, err := exec.Command("ps", "-axo", "pid=,ppid=,cputime=,command=").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		fs := strings.Fields(line)
		if len(fs) < 4 {
			continue
		}
		pid, e1 := strconv.Atoi(fs[0])
		ppid, e2 := strconv.Atoi(fs[1])
		if e1 != nil || e2 != nil {
			continue
		}
		// cputime has no internal spaces (e.g. "12:34.56"), so the command is the
		// remainder. A field that won't parse → cpu 0 (just no CPU signal).
		out[pid] = procInfo{ppid: ppid, cpu: parseCPUTime(fs[2]), command: strings.Join(fs[3:], " ")}
	}
	return out
}

// parseCPUTime parses `ps cputime` ("[[HH:]MM:]SS.cc") into seconds. 0 on error.
func parseCPUTime(s string) float64 {
	parts := strings.Split(s, ":")
	var total float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0
		}
		total = total*60 + v
	}
	return total
}

// subtreeCPU sums cumulative CPU seconds over panePid's process subtree — the
// "how much is this pane computing" input to the hook-free working signal.
func subtreeCPU(panePid int, procs map[int]procInfo, children map[int][]int) float64 {
	var total float64
	seen := map[int]bool{}
	queue := []int{panePid}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if info, ok := procs[pid]; ok {
			total += info.cpu
		}
		queue = append(queue, children[pid]...)
	}
	return total
}

// agentFromCommand matches a full command line against profiles by the basename
// of each argv token, so `node /usr/.../bin/codex` resolves to "Codex" even
// though the pane's foreground command is just "node". Returns the profile name.
func agentFromCommand(command string, profiles []agentProfile) string {
	for idx, tok := range strings.Fields(command) {
		// Only the executable (first token) or a path token (has '/') — so a bare
		// filename argument like `cat codex` doesn't false-match.
		if idx != 0 && !strings.Contains(tok, "/") {
			continue
		}
		base := tok
		if i := strings.LastIndexByte(base, '/'); i >= 0 {
			base = base[i+1:]
		}
		for i := range profiles {
			for _, c := range profiles[i].Commands {
				if base == c {
					return profiles[i].Name
				}
			}
		}
	}
	return ""
}

// agentInSubtree walks panePid's process subtree (using a prebuilt child index)
// and returns the agent name if any process invokes a known agent, else "".
// This is what catches an idle agent whose pane shows comm=node + a plain title.
func agentInSubtree(panePid int, procs map[int]procInfo, children map[int][]int, profiles []agentProfile) string {
	seen := map[int]bool{}
	queue := []int{panePid}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if info, ok := procs[pid]; ok {
			if name := agentFromCommand(info.command, profiles); name != "" {
				return name
			}
		}
		queue = append(queue, children[pid]...)
	}
	return ""
}

// gatherAgents polls every pane and returns the LIVE coding agents, sorted
// waiting → working → idle, then by location. Shared by static + watch.
// A pane is "waiting" (blocked on your input) when claude-notify recorded a
// Notification for it (~/.local/share/gtmux/waiting/<pane>) and it isn't working.
func gatherAgents() []agentPane {
	profiles := loadProfiles()
	lastFinished := state.ReadLastFinished()
	waiting := state.WaitingSet()
	now := time.Now().Unix()

	// One process snapshot per gather, so we can look inside a pane's tree to
	// catch agents that run as `node …/codex` (comm=node, no title glyph).
	procs := snapshotProcs()
	children := map[int][]int{}
	for pid, info := range procs {
		children[info.ppid] = append(children[info.ppid], pid)
	}

	fields := "#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t" +
		"#{pane_title}\t#{pane_current_command}\t#{window_activity_flag}\t#{window_activity}\t#{pane_pid}"
	var panes []agentPane
	for _, line := range tmux.Lines("list-panes", "-a", "-F", fields) {
		f := strings.SplitN(line, "\t", 9)
		if len(f) < 7 {
			continue
		}
		isAgent, agent, status, task := classifyAgent(f[4], f[5], profiles)
		// The title/command can leave a pane unidentified (idle Codex as `node`,
		// no glyph) OR identified-but-unnamed (a WORKING Codex: a spinner title set
		// the status, but cmd=node + no name in the title → generic "agent"). The
		// pane's process tree resolves the real agent in both cases.
		if len(f) >= 9 {
			unnamed := agent == "" || agent == i18n.Tr("agent", "agent")
			if !isAgent || unnamed {
				if panePid, err := strconv.Atoi(f[8]); err == nil {
					if name := agentInSubtree(panePid, procs, children, profiles); name != "" {
						agent = name
						if !isAgent {
							// Process-tree agent with no title spinner (e.g. Codex sets
							// no title): tell working from idle with two hook-free
							// signals OR'd together — the screen is changing (frame),
							// or its process subtree is burning CPU (a local tool
							// running quietly). Both are sampled every poll to keep
							// their baselines fresh.
							frameW := state.PaneFrameWorking(f[0], tmux.CapturePane(f[0]), now)
							cpuW := state.PaneCPUWorking(f[0], subtreeCPU(panePid, procs, children), now)
							status = "idle"
							if frameW || cpuW {
								status = "working"
							}
							isAgent, task = true, strings.TrimSpace(f[4])
						}
					}
				}
			}
		}
		if !isAgent {
			continue
		}
		id := f[0]
		switch {
		case status == "working":
			if waiting[id] { // resumed working → clear the stale waiting mark
				state.Remove(state.WaitingPath(id))
				delete(waiting, id)
			}
		case waiting[id]:
			status = "waiting" // blocked on the user
		}
		var activityAt int64
		if len(f) >= 8 {
			activityAt, _ = strconv.ParseInt(f[7], 10, 64)
		}
		// since = when the agent entered its CURRENT state, for a "working 7m" /
		// "waiting 11m" duration. Hook markers give the turn/wait start; otherwise
		// fall back to last activity.
		since := activityAt
		switch status {
		case "working":
			if mt := fileMtime(state.ActivePath(id)); mt > 0 {
				since = mt
			}
		case "waiting":
			if mt := fileMtime(state.WaitingPath(id)); mt > 0 {
				since = mt
			}
		}
		panes = append(panes, agentPane{
			paneID:     id,
			session:    f[1],
			window:     f[2],
			pane:       f[3],
			loc:        fmt.Sprintf("%s:%s.%s", f[1], f[2], f[3]),
			agent:      agent,
			task:       task,
			status:     status,
			activity:   f[6] == "1",
			latest:     id == lastFinished && status != "working" && status != "waiting",
			source:     "tmux",
			icon:       iconFor(agent, profiles),
			activityAt: activityAt,
			since:      since,
		})
	}
	sort.SliceStable(panes, func(i, j int) bool {
		if ri, rj := statusRank(panes[i].status), statusRank(panes[j].status); ri != rj {
			return ri < rj
		}
		return panes[i].loc < panes[j].loc
	})
	return panes
}

func statusRank(s string) int {
	switch s {
	case "waiting":
		return 0 // needs you now — most urgent
	case "working":
		return 1
	default:
		return 2
	}
}

// agentsSummary renders "N agents · [X waiting ·] Y working · Z idle".
func agentsSummary(panes []agentPane) string {
	s := i18n.Pl(len(panes), "agent")
	if len(panes) == 0 {
		return s
	}
	var nWork, nWait int
	for _, p := range panes {
		switch p.status {
		case "working":
			nWork++
		case "waiting":
			nWait++
		}
	}
	parts := []string{}
	if nWait > 0 {
		parts = append(parts, fmt.Sprintf(i18n.Tr("%d waiting", "%d 等输入"), nWait))
	}
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d working", "%d 运行中"), nWork))
	parts = append(parts, fmt.Sprintf(i18n.Tr("%d idle", "%d 空闲"), len(panes)-nWork-nWait))
	return s + " · " + strings.Join(parts, " · ")
}

// cmdAgents implements `gtmux agents [--watch] [--popup] [--json]`.
func cmdAgents(args []string) int {
	watch, popup, asJSON := false, false, false
	for _, a := range args {
		switch a {
		case "-h", "--help":
			usage()
			return 0
		case "--watch", "-w":
			watch = true
		case "--popup":
			popup = true // close the TUI after a jump (used by the prefix+a popup)
		case "--json":
			asJSON = true
		}
	}
	if !tmux.ServerUp() {
		if asJSON {
			fmt.Println("[]")
			return 0
		}
		i18n.Say("No tmux server running", "没有运行中的 tmux server")
		return 1
	}
	if asJSON {
		return agentsJSON()
	}
	if watch {
		return runWatch(popup)
	}

	panes := gatherAgents()
	fmt.Printf("%sgtmux %s%s — %s\n\n", i18n.Bold, i18n.Tr("agents", "agent"), i18n.Reset, agentsSummary(panes))
	if len(panes) == 0 {
		i18n.Say("No coding-agent panes found.", "没有发现 coding-agent 的 pane。")
		return 0
	}
	for _, p := range panes {
		glyph, color, label := statusStyle(p.status)
		task := p.task
		if task == "" {
			task = i18n.Dim + "—" + i18n.Reset
		}
		dot := ""
		if p.activity {
			dot = i18n.Yellow + " •" + i18n.Reset
		}
		done := ""
		if p.latest {
			done = i18n.Yellow + i18n.Tr("  ✓ latest", "  ✓ 最近完成") + i18n.Reset
		}
		fmt.Printf("%s%s%s %s%s%s %s%s%s %s%s%s %s%s%s%s\n",
			color, glyph, i18n.Reset,
			color, i18n.PadRight(label, 8), i18n.Reset,
			i18n.Bold, i18n.PadRight(p.agent, 12), i18n.Reset,
			i18n.Bold, i18n.PadRight(p.loc, 22), i18n.Reset,
			task, dot, i18n.Dim+" "+p.paneID+i18n.Reset, done)
	}
	fmt.Printf("\n%s%s%s\n", i18n.Dim,
		i18n.Tr("jump: gtmux focus <pane>   (e.g. gtmux focus "+panes[0].paneID+")",
			"跳转：gtmux focus <pane>   （例如 gtmux focus "+panes[0].paneID+"）"), i18n.Reset)
	return 0
}

// agentsJSONBytes marshals the live agents into the stable `gtmux agents --json`
// array (no colors, no screen-scraping). Shared by the CLI command and the
// remote server (internal/server) so both speak one byte-identical contract.
func agentsJSONBytes() ([]byte, error) {
	panes := gatherAgents()
	out := make([]agentJSON, 0, len(panes))
	for _, p := range panes {
		src := p.source
		if src == "" {
			src = "tmux"
		}
		out = append(out, agentJSON{
			PaneID: p.paneID, Session: p.session, Window: p.window, Pane: p.pane,
			Loc: p.loc, Agent: p.agent, Status: p.status, Task: p.task,
			Latest: p.latest, Activity: p.activity,
			Source: src, Project: p.project, Terminal: p.terminal, Tab: p.tab,
			ActivityAt: p.activityAt, Since: p.since, Icon: p.icon,
		})
	}
	return json.MarshalIndent(out, "", "  ")
}

// agentsJSON prints the live agents as a JSON array (stable shape; no colors,
// no screen-scraping — for scripts and the menu-bar app).
func agentsJSON() int {
	b, err := agentsJSONBytes()
	if err != nil {
		i18n.Sae("json error: "+err.Error(), "json 错误："+err.Error())
		return 1
	}
	fmt.Println(string(b))
	return 0
}

func statusStyle(status string) (glyph, color, label string) {
	switch status {
	case "working":
		return "⠿", i18n.Cyan, i18n.Tr("working", "运行中")
	case "waiting":
		return "⏸", i18n.Yellow, i18n.Tr("waiting", "等输入")
	case "idle":
		return "✳", i18n.Green, i18n.Tr("idle", "空闲")
	default:
		return "●", i18n.Yellow, i18n.Tr("running", "运行中")
	}
}
