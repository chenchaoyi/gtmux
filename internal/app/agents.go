package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
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
	branch     string // git branch of the pane's cwd (radar++), "" if not a repo
	terminal   string
	tab        string
	activityAt int64  // epoch seconds of last activity (relative time)
	since      int64  // epoch seconds the current state began (for a duration)
	icon       string // identity icon hint (.app path or image path); "" = monogram
	// native (source=="native") only: the agent session id (adopt key) + whether
	// the agent can be resumed into tmux (so surfaces can hide Adopt otherwise).
	sessionID string
	adoptable bool
	// errored-idle modifier: this idle session ended on an API/tool error.
	errored   bool
	errorText string
	// background-running modifier: this idle session's turn ended with background
	// work (a run_in_background shell, …) still in flight — "paused waiting for
	// background work", not truly done. count + a short label.
	bg      bool
	bgCount int
	bgText  string
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
	Project    string `json:"project,omitempty"`  // repo root basename (tmux: cwd; native: cwd)
	Branch     string `json:"branch,omitempty"`   // git branch of the pane's cwd (radar++)
	Terminal   string `json:"terminal,omitempty"` // native: terminal app
	Tab        string `json:"tab,omitempty"`      // native: terminal tab title (jump key)
	ActivityAt int64  `json:"activity_at,omitempty"`
	Since      int64  `json:"since,omitempty"` // epoch the current state began (duration)
	Icon       string `json:"icon,omitempty"`  // identity icon hint (.app/image path)
	// native rows only: the agent session id (the `gtmux adopt <id>` key) + whether
	// it can be adopted into tmux (resumable). Absent/false for tmux rows.
	SessionID string `json:"session_id,omitempty"`
	Adoptable bool   `json:"adoptable,omitempty"`
	// errored-idle modifier: an idle session whose LAST transcript message was an
	// API/tool error (it ended on a failure, not a clean finish). Surfaces mark it
	// with an amber ⚠ (NOT red — red is `waiting`). Absent/false = finished normally.
	Error     bool   `json:"error,omitempty"`
	ErrorText string `json:"error_text,omitempty"` // short summary for the row
	// background-running modifier: an idle session whose turn ended with background
	// work still in flight (Claude's Stop-payload background_tasks). Surfaces mark it
	// with an amber ⧗ (NOT red). Absent/false = truly finished. bg_count = how many.
	Bg      bool   `json:"bg,omitempty"`
	BgCount int    `json:"bg_count,omitempty"`
	BgText  string `json:"bg_text,omitempty"` // short label (e.g. the shell command)
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

// gitInfo resolves the git project (repo-root basename) and branch for a pane's
// working directory by reading the repo metadata directly — NO git subprocess,
// so the CLI stays cgo-free and cheap to call per pane every poll. It walks up
// from cwd to the first ancestor that has a .git entry; project is that dir's
// basename, branch is parsed from its HEAD. Returns ("", "") for a non-repo cwd.
func gitInfo(cwd string) (project, branch string) {
	if cwd == "" {
		return "", ""
	}
	dir := cwd
	for i := 0; i < 64; i++ {
		gitPath := filepath.Join(dir, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			return filepath.Base(dir), headBranch(gitPath, fi.IsDir())
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ""
}

// headBranch parses the branch name from a .git location. gitPath is either a
// directory (normal repo) or a file (worktree/submodule: "gitdir: <path>"). It
// reads HEAD: "ref: refs/heads/<branch>" → branch; a detached HEAD (raw SHA) →
// short SHA. Returns "" if anything is unreadable.
func headBranch(gitPath string, isDir bool) string {
	gitDir := gitPath
	if !isDir {
		b, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(b))
		const p = "gitdir:"
		if !strings.HasPrefix(line, p) {
			return ""
		}
		gitDir = strings.TrimSpace(strings.TrimPrefix(line, p))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(filepath.Dir(gitPath), gitDir)
		}
	}
	b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	const ref = "ref: refs/heads/"
	if strings.HasPrefix(head, ref) {
		return strings.TrimPrefix(head, ref)
	}
	if len(head) >= 7 { // detached HEAD → short SHA
		return head[:7]
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
		"#{pane_title}\t#{pane_current_command}\t#{window_activity_flag}\t#{window_activity}\t#{pane_pid}\t#{pane_current_path}"
	var panes []agentPane
	for _, line := range tmux.Lines("list-panes", "-a", "-F", fields) {
		f := strings.SplitN(line, "\t", 10)
		if len(f) < 7 {
			continue
		}
		isAgent, agent, status, task := classifyAgent(f[4], f[5], profiles)
		// hookFreeStatus tells WORKING from IDLE for an agent whose title can't (Codex
		// sets no idle glyph like Claude's ✳): working if the screen is changing (frame)
		// OR its process subtree is burning CPU (a local tool running quietly), else
		// idle. Both are sampled every poll to keep their baselines fresh.
		hookFreeStatus := func(paneID string, panePid int) string {
			frameW := state.PaneFrameWorking(paneID, tmux.CapturePane(paneID), now)
			cpuW := state.PaneCPUWorking(paneID, subtreeCPU(panePid, procs, children), now)
			if frameW || cpuW {
				return "working"
			}
			return "idle"
		}
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
							status = hookFreeStatus(f[0], panePid)
							isAgent, task = true, strings.TrimSpace(f[4])
						}
					}
				}
			}
		}
		if !isAgent {
			continue
		}
		// A live but glyph-less agent (Codex) that we DID identify comes back as bare
		// "running" — classifyAgent can't tell idle from working without an idle glyph.
		// Refine it the same hook-free way, so a FINISHED Codex shows idle (✓) rather
		// than a grey "running" dot. (Claude's ✳ already resolves this upstream.)
		if status == "running" && len(f) >= 9 {
			if panePid, err := strconv.Atoi(f[8]); err == nil {
				status = hookFreeStatus(f[0], panePid)
			}
		}
		id := f[0]
		var activityAt int64
		if len(f) >= 8 {
			activityAt, _ = strconv.ParseInt(f[7], 10, 64)
		}
		switch {
		case status == "working":
			if waiting[id] { // resumed working → clear the stale waiting mark
				state.Remove(state.WaitingPath(id))
				delete(waiting, id)
			}
		case waiting[id] && !waitMarkStale(id, activityAt):
			status = "waiting" // blocked on the user (hook-marked, e.g. Claude)
		default:
			// A waiting mark survives on disk across a tmux restart; when pane ids
			// reset and a new session reuses this id, it inherits a days-old orphan
			// mark (the "waiting 9d" bug). Clear any mark on a non-working pane whose
			// activity postdates it — the agent has clearly moved on.
			if waiting[id] {
				state.Remove(state.WaitingPath(id))
				delete(waiting, id)
			}
			// "Waiting" is HOOK-DRIVEN ONLY (the marker set above), never inferred from
			// screen output. A screen-scan fallback used to promote a pane to waiting
			// when a numbered menu appeared on screen — but it false-alarmed on ordinary
			// prose (a "1. … 2. …" list in an agent's own message read as an approval
			// menu), popping a bogus approval card on an idle session. The waiting state
			// belongs to the hook/session, not to whatever text is on the terminal.
		}
		// since = when the agent entered its CURRENT state, for a "working 7m" /
		// "waiting 11m" / "idle 3m" duration. Hook markers give the turn/wait/finish
		// start; otherwise fall back to last activity.
		since := activityAt
		var errored bool
		var errorText string
		var bg bool
		var bgCount int
		var bgText string
		switch status {
		case "working":
			if mt := fileMtime(state.ActivePath(id)); mt > 0 {
				since = mt
			}
		case "waiting":
			if mt := fileMtime(state.WaitingPath(id)); mt > 0 {
				since = mt
			}
		case "idle":
			// Prefer when the turn actually FINISHED over last window activity — a
			// live agent status line keeps redrawing, which bumps window-activity
			// every couple seconds and would otherwise reset the idle time to ~0s on
			// every poll. The Stop hook stamps finished/<pane> at the real finish; if
			// it's absent (the agent fired no Stop, or finished before this shipped),
			// materialize one from the agent's transcript-log mtime (last real write ≈
			// when it finished, immune to redraws), else last window activity. Only ever
			// CREATED here (the hook clears it on the next turn), so an idle↔working flap
			// from a redraw can't keep resetting it, and each pane keeps its own time.
			fp := state.FinishedPath(id)
			mt := fileMtime(fp)
			if mt == 0 {
				finishedAt := activityAt
				// This pane's OWN session (resume.Load maps loc→sessionId) → the last
				// message it logged, which is the real finish time. NOT the log FILE
				// mtime (a resume rewrites the file without new messages) nor the
				// newest log in the cwd (a different session may have run there since).
				loc := fmt.Sprintf("%s:%s.%s", f[1], f[2], f[3])
				if rec, ok := resume.Load(loc); ok {
					if t := transcript.LastMessageTime(rec.Agent, rec.SessionID); t > 0 {
						finishedAt = t
					}
				}
				_ = state.Touch(fp)
				if finishedAt > 0 {
					_ = os.Chtimes(fp, time.Unix(finishedAt, 0), time.Unix(finishedAt, 0))
				}
				mt = finishedAt
			}
			if mt > 0 {
				since = mt
			}
			// errored-idle: did this session END on an API/tool error (last transcript
			// message is isApiErrorMessage)? Mark it so surfaces show ⚠ not ✓.
			loc := fmt.Sprintf("%s:%s.%s", f[1], f[2], f[3])
			if rec, ok := resume.Load(loc); ok {
				if e, txt := transcript.LastMessageError(rec.Agent, rec.SessionID); e {
					errored, errorText = true, txt
				}
			}
			// background-running: the Stop hook stamped bg/<pane> because the turn
			// ended with background work still in flight → mark the idle row ⧗ (not ✓).
			if n, label := state.ReadBackground(id); n > 0 {
				bg, bgCount, bgText = true, n, label
			}
		}
		// radar++ : the pane's cwd → git repo root basename (project) + branch,
		// read straight from .git (no git subprocess; cgo-free). "" if not a repo.
		var project, branch string
		if len(f) >= 10 {
			project, branch = gitInfo(f[9])
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
			project:    project,
			branch:     branch,
			icon:       iconFor(agent, profiles),
			activityAt: activityAt,
			since:      since,
			errored:    errored,
			errorText:  errorText,
			bg:         bg,
			bgCount:    bgCount,
			bgText:     bgText,
		})
	}
	// Sensed non-tmux (native) sessions: hook-tracked, no pane to view/jump/send.
	panes = append(panes, nativePanes(panes, profiles, now)...)
	sortPanes(panes)
	return panes
}

// nativePanes turns the native-session store into `source:"native"` radar rows —
// sensed agents running outside tmux (no pane, so no focus/jump/send). A native
// session that's ALSO live in tmux (its session id matches a tmux pane's resume
// record — e.g. it was adopted) is suppressed; the tmux row wins.
func nativePanes(tmuxPanes []agentPane, profiles []agentProfile, now int64) []agentPane {
	recs := native.Live(now)
	if len(recs) == 0 {
		return nil
	}
	inTmux := map[string]bool{}
	for _, p := range tmuxPanes {
		if rec, ok := resume.Load(p.loc); ok && rec.SessionID != "" {
			inTmux[rec.SessionID] = true
		}
	}
	var out []agentPane
	for _, r := range recs {
		if inTmux[r.SessionID] {
			continue
		}
		name, icon := displayForKey(r.Agent, profiles)
		project, branch := gitInfo(r.Cwd)
		// The session's own last logged message (tmux-free); 0 when nothing is
		// persisted yet. Drives the idle "finished N ago" AND gates Adopt: a session
		// with no on-disk conversation can't be resumed ("No conversation found"), so
		// don't offer Adopt for it.
		lastMsg := transcript.LastMessageTime(r.Agent, r.SessionID)
		since := r.UpdatedAt
		if r.State == "idle" && lastMsg > 0 {
			since = lastMsg
		}
		out = append(out, agentPane{
			agent: name, status: r.State, source: "native",
			project: project, branch: branch, icon: icon,
			activityAt: r.UpdatedAt, since: since,
			// Adopt only an IDLE, resumable session with a real on-disk conversation —
			// never one mid-turn (working): resuming it would fight the live instance.
			sessionID: r.SessionID, adoptable: r.State == "idle" && resume.Resumable(r.Agent) && lastMsg > 0,
		})
	}
	return out
}

// displayForKey maps a hook agent key (claude/codex/…) to its profile display
// name + icon (matching on the profile's command list), else the raw key.
func displayForKey(key string, profiles []agentProfile) (name, icon string) {
	for _, p := range profiles {
		for _, c := range p.Commands {
			if c == key {
				return p.Name, p.Icon
			}
		}
	}
	return key, ""
}

// sortPanes orders the radar: status groups first (waiting → working → idle/running),
// then within the FINISHED (idle) group most-recently-finished first (its `since` is
// frozen at last activity, so the order stays stable — no jumping), and every other
// group by location (a stable, familiar layout).
func sortPanes(panes []agentPane) {
	sort.SliceStable(panes, func(i, j int) bool {
		if ri, rj := statusRank(panes[i].status), statusRank(panes[j].status); ri != rj {
			return ri < rj
		}
		if panes[i].status == "idle" && panes[j].status == "idle" && panes[i].since != panes[j].since {
			return panes[i].since > panes[j].since
		}
		return panes[i].loc < panes[j].loc
	})
}

// waitStaleGrace is how far a pane's activity may postdate its waiting mark before
// the mark is treated as obsolete. A real wait leaves the pane quiet (activity ≈ the
// mark); activity well after it means the agent resumed, or the pane id was reused
// across a tmux restart and inherited an orphan mark from a prior incarnation.
const waitStaleGrace int64 = 15 * 60 // seconds

// waitMarkStale reports whether a pane's waiting mark is obsolete — the pane's tmux
// window activity is newer than the mark by more than the grace. Guards against the
// "waiting 9d" orphan-mark case (window_activity is coarse, hence the grace).
func waitMarkStale(id string, activityAt int64) bool {
	if activityAt <= 0 {
		return false
	}
	mt := fileMtime(state.WaitingPath(id))
	return mt > 0 && activityAt > mt+waitStaleGrace
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
		// errored idle: an amber ⚠ modifier (NOT red — red is waiting), and surface
		// the failure summary as the task so "ended on an error" is visible at a glance.
		if p.errored {
			glyph, color, label = "⚠", i18n.Amber, i18n.Tr("errored", "报错")
			if p.errorText != "" {
				task = i18n.Amber + p.errorText + i18n.Reset
			}
		}
		// background-running idle: keep the green ✓ idle glyph (it IS idle), but flag
		// with an amber ⧗ modifier (NOT red) that background work is still in flight.
		if p.bg && !p.errored {
			n := ""
			if p.bgCount > 1 {
				n = fmt.Sprintf("%d", p.bgCount)
			}
			mark := i18n.Amber + "⧗" + n + " " + i18n.Tr("background running", "后台运行中") + i18n.Reset
			if p.bgText != "" {
				task = mark + i18n.Dim + " · " + p.bgText + i18n.Reset
			} else {
				task = mark
			}
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
			Source: src, Project: p.project, Branch: p.branch, Terminal: p.terminal, Tab: p.tab,
			ActivityAt: p.activityAt, Since: p.since, Icon: p.icon,
			SessionID: p.sessionID, Adoptable: p.adoptable,
			Error: p.errored, ErrorText: p.errorText,
			Bg: p.bg, BgCount: p.bgCount, BgText: p.bgText,
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
