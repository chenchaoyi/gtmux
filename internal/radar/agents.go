// Package radar is the pane-data kernel: it polls tmux (and the native-session
// store) and assembles the live coding-agent radar — the `gtmux agents --json`
// rows, the per-agent digest, and the usage report. It depends ONLY on leaf
// packages (tmux, dispatch, prompt, resume, state, native, transcript, i18n,
// limits, resource, usage); it never imports the command layer (internal/app)
// or the supervisor (internal/hq), so those may depend on it without a cycle.
package radar

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/assets"
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// StuckDispatchKind reports why a TRACKED dispatch pane is stuck BEFORE running a turn —
// "startup" (an agent startup/permission gate, per-agent) or "draft" (its goal pasted
// but left unsubmitted in the composer) — or "" when it isn't. Only tracked dispatches
// are inspected (a human mid-compose in a normal pane is never flagged). Pure read of a
// single capture; the caller decides what to do with it.
func StuckDispatchKind(paneID, agent string) string {
	if _, ok := dispatch.TaskForPane(paneID); !ok {
		return "" // not a tracked dispatch — skip the captures entirely
	}
	return classifyStuck(tmux.CaptureFull(paneID), tmux.CaptureFullColor(paneID), agent, true)
}

// classifyStuck is the pure decision behind StuckDispatchKind: given a pane's plain and
// COLOR captures, the agent name, and whether it's a tracked dispatch, return "startup"
// / "draft" / "". Separated from the tmux reads so the gate/draft classification is
// unit-testable. The draft check is COLOR-aware: a plain capture can't tell a real
// unsubmitted draft from CC's faint suggested-next-command ghost text, so DraftOfColored
// drops the faint (SGR 2) ghost before reading the box (else a false-positive `draft`).
func classifyStuck(plainCap, colorCap, agent string, tracked bool) string {
	if !tracked {
		return ""
	}
	if prompt.IsStartupGate(plainCap, agent) {
		return "startup"
	}
	if draft, structured := dispatch.DraftOfColored(colorCap); structured && strings.TrimSpace(draft) != "" {
		return "draft"
	}
	return ""
}

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
// builtinProfiles is derived from the agent registry (the single source of
// truth); it maps each registry Profile to the radar's agentProfile shape.
var builtinProfiles = func() []agentProfile {
	ps := agents.Profiles()
	out := make([]agentProfile, len(ps))
	for i, p := range ps {
		out[i] = agentProfile{Name: p.Label, Commands: p.Commands, IdleGlyph: p.IdleGlyph, Icon: p.Icon}
	}
	return out
}()

func LoadProfiles() []agentProfile {
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

// FileMtime returns a file's modtime in unix seconds, 0 if it doesn't exist.
func FileMtime(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime().Unix()
	}
	return 0
}

// IconFor returns the icon hint for the agent named name (first matching profile,
// so user overrides win). When a profile has no icon path but the binary ships a
// committed built-in icon for that agent, it returns a "builtin:<key>" hint so the
// icon field is NON-EMPTY — surfaces that gate the /api/icon fetch on a truthy
// `icon` (the mobile AgentAvatar) then still fetch it, and the serve serves the
// committed PNG. "" only when there is no icon of any kind.
func IconFor(name string, profiles []agentProfile) string {
	for i := range profiles {
		if profiles[i].Name == name {
			if profiles[i].Icon != "" {
				return profiles[i].Icon
			}
			break
		}
	}
	if key := agents.KeyForLabel(name); key != "" && assets.AgentIcon(key) != nil {
		return "builtin:" + key
	}
	return ""
}

type Pane struct {
	PaneID   string
	session  string
	window   string // window index
	pane     string // pane index
	Loc      string // session:window.pane
	Agent    string // display name, "" if unknown type
	Task     string // title with the status glyph stripped
	Status   string // "working" | "waiting" | "idle" | "running"
	Activity bool
	Latest   bool // the most-recently-finished pane (claude-notify last-finished)
	// terminal generalization (DESIGN §7)
	source     string // "tmux" | "native"
	cwd        string // the pane's working dir (drives project/branch + supervisor detection)
	role       string // "supervisor" when this is the hq (中控) session, else ""
	project    string
	branch     string // git branch of the pane's cwd (radar++), "" if not a repo
	terminal   string
	tab        string
	activityAt int64  // epoch seconds of last activity (relative time)
	Since      int64  // epoch seconds the current state began (for a duration)
	icon       string // identity icon hint (.app path or image path); "" = monogram
	// input-lock modifier: the pane is in tmux copy/view-mode (the user is scrolling
	// the scrollback), so typed input — manual OR gtmux send/spawn — is swallowed as
	// mode-nav until it exits. Surfaces flag which pane is input-locked. tmux rows only.
	inMode bool
	// native (source=="native") only: the agent session id (adopt key) + whether
	// the agent can be resumed into tmux (so surfaces can hide Adopt otherwise).
	sessionID string
	adoptable bool
	// errored-idle modifier: this idle session ended on an API/tool error.
	Errored   bool
	ErrorText string
	// background-running modifier: this idle session's turn ended with background
	// work (a run_in_background shell, …) still in flight — "paused waiting for
	// background work", not truly done. count + a short label.
	Bg      bool
	BgCount int
	BgText  string
	// usage-watch modifier: this session breached (or projects into) a usage
	// layer — content of the usagewarn marker, e.g. "ctx 86%". Amber, like bg.
	usageWarn string
	// watched (tiered-pane-control): this row is a PLAIN pane the user opted onto the
	// radar via `gtmux watch`. It is NOT an agent — it carries no agent status
	// (waiting/working/idle), just "a pane you asked to keep an eye on". Never
	// auto-added; dropped when its pane closes.
	Watched bool
}

// Role exposes the pane's role ("supervisor" for HQ, else "") to callers outside
// the package (e.g. serve threading it into the fleet snapshot for role-gating).
func (p Pane) Role() string { return p.role }

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
	Source string `json:"source"` // "tmux" | "native"
	// Role marks special sessions; the only value today is "supervisor" — the hq
	// (中控) session, detected by its pane cwd being the hq home (rename-proof).
	// Additive + omitempty: absent for normal agents, so consumers are unaffected.
	Role       string `json:"role,omitempty"`
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
	// usage-watch modifier (usage-watch change): a breached/projected usage layer,
	// e.g. "ctx 86%" / "burn→5M in ~12m". Amber modifier, never a status.
	UsageWarn string `json:"usage_warn,omitempty"`
	// input-lock modifier: the pane is in tmux copy/view-mode, so typed input is
	// swallowed until it exits. `gtmux send`/`spawn` auto-exit before delivering;
	// this flag lets surfaces show WHICH pane is input-locked. Absent = not in a mode.
	InMode bool `json:"in_mode,omitempty"`
	// watched (tiered-pane-control): this row is a user-promoted PLAIN pane, not a
	// coding agent — it carries no agent status. Surfaces render it as a distinct
	// watched row. Additive + omitempty: absent for agent rows.
	Watched bool `json:"watched,omitempty"`
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

// psSnapshotTimeout bounds the full-table `ps`. A normal run is <100ms; 4s is far
// above any healthy machine yet short enough that a hung poll degrades fast.
const psSnapshotTimeout = 4 * time.Second

// boundedPS runs `ps <args>` with a hard timeout and ABANDONS it if it overruns.
// `ps -axo …command=` reads every process's argv (KERN_PROCARGS2); if ANY process is
// stuck in an uninterruptible kernel wait — a corporate VPN/EDR agent wedged during a
// network switch has done exactly this — that read blocks indefinitely, and the wedged
// reader can't even be killed (SIGKILL/SIGALRM stay pending on a U-state process). The
// context timeout cancels + kills, and WaitDelay makes Output() RETURN after the delay by
// closing the I/O pipes rather than waiting forever on an unreapable child. Without this
// one wedged process froze the whole menu bar (which polls this every refresh) and piled
// up 100+ stuck `ps`. The abandoned `ps` is reparented to launchd and reaped when it
// finally unblocks; the caller degrades (empty snapshot) instead of hanging.
func boundedPS(args ...string) ([]byte, error) {
	return boundedOutput(psSnapshotTimeout, "ps", args...)
}

// boundedOutput runs name+args and returns its stdout, but truly ABANDONS it if it
// overruns timeout. It must survive a child stuck in uninterruptible kernel sleep (a
// wedged VPN/EDR agent makes `ps` do this) — and such a child can't be killed
// (SIGKILL stays pending) nor reaped (wait4 never returns). CommandContext+WaitDelay is
// NOT enough: WaitDelay closes the I/O pipes but Cmd.Wait still blocks in wait4 on the
// unreapable child. So we read + Wait in a goroutine and select against a timer: on
// timeout we best-effort Kill and RETURN, leaving the goroutine parked in the (buffered)
// send until the child eventually dies and is reaped — no leak of anything but one parked
// goroutine, and the CALLER never blocks.
func boundedOutput(timeout time.Duration, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	type result struct {
		b   []byte
		err error
	}
	done := make(chan result, 1) // buffered so the goroutine never blocks sending
	go func() {
		b, _ := io.ReadAll(stdout)
		done <- result{b, cmd.Wait()}
	}()
	select {
	case r := <-done:
		return r.b, r.err
	case <-time.After(timeout):
		_ = cmd.Process.Kill() // best-effort; a U-state process ignores it until it unblocks
		return nil, fmt.Errorf("%s timed out after %s", name, timeout)
	}
}

// snapshotProcs returns pid → {ppid, cpu, argv} for every process (one `ps`
// call), so we can look inside a pane's process tree and sum its CPU. Empty on
// any failure OR a timeout — a degraded snapshot loses only the CPU "working"
// signal and bare-`node` agent detection; title-identified agents still appear.
func snapshotProcs() map[int]procInfo {
	out := map[int]procInfo{}
	b, err := boundedPS("-axo", "pid=,ppid=,cputime=,command=")
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

// GatherAgents polls every pane and returns the LIVE coding agents, sorted
// waiting → working → idle, then by location. Shared by static + watch.
// A pane is "waiting" (blocked on your input) when claude-notify recorded a
// Notification for it (~/.local/share/gtmux/waiting/<pane>) and it isn't working.
// paneSource lists the raw tmux pane field-lines GatherAgents assembles into radar
// rows — one tab-separated line per pane, in the field order the parser below reads
// (pane_id, session, window, pane, title, command, activity_flag, activity, pane_pid,
// current_path, in_mode). A package var (not a direct tmux call) so fixture tests can
// inject canned panes and drive the whole assemble/join/sort path with no live server.
var paneSource = func() []string {
	const fields = "#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t" +
		"#{pane_title}\t#{pane_current_command}\t#{window_activity_flag}\t#{window_activity}\t#{pane_pid}\t#{pane_current_path}\t#{pane_in_mode}\t#{@gtmux_hq_home}"
	return tmux.Lines("list-panes", "-a", "-F", fields)
}

// procSnapshot is the per-gather process table (pid → {ppid, cpu, argv}). A package
// var so fixture tests can supply a fixed map instead of the live `ps`, keeping the
// glyph-less-agent subtree path deterministic.
var procSnapshot = snapshotProcs

// stripDefaultTitle blanks a pane title that is just tmux's DEFAULT — the machine's
// hostname — so an agent that never sets a terminal title (codex) doesn't surface
// the HOSTNAME as its "session name" on every UI. With the title blanked, Task stays
// empty and the phone/menu-bar row falls back to the tmux session name, which is
// what the user actually named. (Seen live 2026-08-07: a codex row's primary line
// read "MBP-….local".) Matching ignores case and a trailing ".local" on either side
// — macOS reports "Foo.local" while tmux may render either form.
func stripDefaultTitle(title, host string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	h := strings.ToLower(strings.TrimSpace(host))
	if t == "" || h == "" {
		return title
	}
	if strings.TrimSuffix(t, ".local") == strings.TrimSuffix(h, ".local") {
		return ""
	}
	return title
}

func GatherAgents() []Pane {
	profiles := LoadProfiles()
	lastFinished := state.ReadLastFinished()
	waiting := state.WaitingSet()
	now := time.Now().Unix()

	// One process snapshot per gather, so we can look inside a pane's tree to
	// catch agents that run as `node …/codex` (comm=node, no title glyph).
	procs := procSnapshot()
	hostname, _ := os.Hostname() // for stripDefaultTitle (tmux's default pane_title)
	children := map[int][]int{}
	for pid, info := range procs {
		children[info.ppid] = append(children[info.ppid], pid)
	}

	var panes []Pane
	hqStamps := map[string]string{}         // pane id → its @gtmux_hq_home stamp (role precedence)
	livePanes := map[string]bool{}          // every live tmux pane id (agents or not) — for orphan-marker GC
	paneFieldsByID := map[string][]string{} // pane id → its raw field slice (for watched-pane rows)
	seenPane := map[string]bool{}           // pane ids already turned into a row — dedup linked/shared windows
	for _, line := range paneSource() {
		f := strings.SplitN(line, "\t", 12)
		if len(f) < 7 {
			continue
		}
		livePanes[f[0]] = true
		// A window linked into more than one session (or a shared/attached window) is
		// listed by `list-panes -a` ONCE PER SESSION, so the same pane_id arrives on
		// multiple lines with different session/window fields. Emit ONE radar row per
		// pane — the first wins — else the phone showed the same session twice (e.g. a
		// worker appearing under both its own session and a linked "HQ" one). role
		// precedence still resolves correctly afterward (it's keyed by pane_id stamp).
		if seenPane[f[0]] {
			continue
		}
		seenPane[f[0]] = true
		// Blank a hostname-default title IN PLACE so every downstream consumer
		// (classify → Task, watched-pane rows) sees "no title", not the hostname.
		f[4] = stripDefaultTitle(f[4], hostname)
		paneFieldsByID[f[0]] = f
		isAgent, agent, status, task := classifyAgent(f[4], f[5], profiles)
		// hookFreeStatus tells WORKING from IDLE for an agent whose title can't (Codex
		// sets no idle glyph like Claude's ✳): working if the screen is changing (frame)
		// OR its process subtree is burning CPU (a local tool running quietly), else
		// idle. Both are sampled every poll to keep their baselines fresh.
		hfsCalled := false // whether the frame/CPU signal was sampled for this pane
		hookFreeStatus := func(paneID string, panePid int) string {
			hfsCalled = true
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
		panePid := 0
		if len(f) >= 9 {
			panePid, _ = strconv.Atoi(f[8])
		}
		var activityAt int64
		if len(f) >= 8 {
			activityAt, _ = strconv.ParseInt(f[7], 10, 64)
		}
		// liveWorking reports whether the pane is DEMONSTRABLY working right now (screen
		// changing / subtree burning CPU) — the reliable signal, unlike the pane TITLE's
		// braille spinner, which stays frozen on its last frame while the agent is BLOCKED
		// on a prompt. Reuse the frame/CPU sample if it was already taken this pane
		// (glyph-less agents); otherwise sample it here — lazily, so the capture cost is
		// paid only when a fresh mark actually needs disambiguating.
		liveWorking := func() bool {
			if hfsCalled {
				return status == "working"
			}
			return hookFreeStatus(id, panePid) == "working"
		}
		var clearMark bool
		status, clearMark = resolveWaiting(status, waiting[id], waitMarkStale(id, activityAt), liveWorking)
		if clearMark {
			state.Remove(state.WaitingPath(id))
			delete(waiting, id)
		}
		// STUCK-DISPATCH GUARD: a dispatched worker blocked BEFORE running a turn (a
		// startup/permission gate, or its goal left unsubmitted in the composer) fires no
		// hook, so resolveWaiting left it idle/running — and idle → done in `gtmux tasks`,
		// wrongly reporting a task that never ran as finished. If this is a TRACKED
		// dispatch whose screen shows a gate or holds its own undelivered draft, read it
		// as waiting (needs-you). Narrow, hook-free, DISPLAY-only — the serve slow-tick
		// (the single writer) owns the marker + the `waiting` wake.
		if status == "idle" || status == "running" {
			if StuckDispatchKind(id, agent) != "" {
				status = "waiting"
			}
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
			if mt := FileMtime(state.ActivePath(id)); mt > 0 {
				since = mt
			}
		case "waiting":
			if mt := FileMtime(state.WaitingPath(id)); mt > 0 {
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
			mt := FileMtime(fp)
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
		var cwd, project, branch string
		if len(f) >= 10 {
			cwd = f[9]
			project, branch = gitInfo(cwd)
		}
		usageWarn := state.ReadMarker(state.UsageWarnPath(id))
		inMode := len(f) >= 11 && f[10] == "1"
		if len(f) >= 12 && f[11] != "" {
			hqStamps[id] = f[11]
		}
		panes = append(panes, Pane{
			PaneID:     id,
			session:    f[1],
			window:     f[2],
			pane:       f[3],
			Loc:        fmt.Sprintf("%s:%s.%s", f[1], f[2], f[3]),
			Agent:      agent,
			Task:       task,
			Status:     status,
			Activity:   f[6] == "1",
			Latest:     id == lastFinished && status != "working" && status != "waiting",
			source:     "tmux",
			cwd:        cwd,
			role:       roleForCwd(cwd),
			project:    project,
			branch:     branch,
			icon:       IconFor(agent, profiles),
			activityAt: activityAt,
			Since:      since,
			Errored:    errored,
			ErrorText:  errorText,
			Bg:         bg,
			BgCount:    bgCount,
			BgText:     bgText,
			usageWarn:  usageWarn,
			inMode:     inMode,
		})
	}
	// Sensed non-tmux (native) sessions: hook-tracked, no pane to view/jump/send.
	panes = append(panes, nativePanes(panes, profiles, now)...)
	// Opt-in watched plain panes (tiered-pane-control): user-promoted non-agent panes,
	// appended as distinct watched rows (no agent status). Built from the raw pane
	// scan so we know their loc/cwd/command; only panes NOT already an agent row and
	// still live are added.
	panes = append(panes, watchedPanes(panes, paneFieldsByID)...)
	applyRolePrecedence(panes, hqStamps)
	// Self-heal: drop turn-state markers left behind by panes tmux no longer has
	// (an exited agent whose pane was closed). livePanes is the raw pane set gathered
	// above (agents or not) — reusing it avoids a second list-panes on the hot path.
	// Guard on non-empty: a transient empty scan must never reap every marker.
	if len(livePanes) > 0 {
		state.ReapOrphanTurnMarkers(livePanes)
	}
	sortPanes(panes)
	return panes
}

// applyRolePrecedence enforces that the supervisor role belongs to ONE pane
// (hq-home-quarantine): when any pane carries the `gtmux hq` stamp for THIS home,
// the stamp is the sole authority — a worker session parked in the HQ home (a cwd
// match) stays a NORMAL, visible radar row instead of masquerading as HQ. The cwd
// fallback stands only when no stamped pane exists (legacy homes predating the
// stamp).
func applyRolePrecedence(panes []Pane, stamps map[string]string) {
	home := state.HQHome()
	stamped := false
	for _, v := range stamps {
		if hqpane.SameDir(v, home) {
			stamped = true
			break
		}
	}
	if !stamped {
		return
	}
	for i := range panes {
		if hqpane.SameDir(stamps[panes[i].PaneID], home) {
			panes[i].role = "supervisor"
		} else {
			panes[i].role = ""
		}
	}
}

// watchedPanes turns user-watched PLAIN panes into distinct watched radar rows
// (tiered-pane-control). A watched pane that is ALSO an agent (`existing` already has
// it) is skipped — the agent row wins; you don't double-list it. A watched pane whose
// pane is no longer live (absent from paneFieldsByID) is skipped here and its marker
// is reaped by the orphan sweep. A watched row carries NO agent status: it's "a pane
// you asked to keep an eye on", labeled by its command/title, not waiting/working/idle.
func watchedPanes(existing []Pane, paneFieldsByID map[string][]string) []Pane {
	watched := state.WatchedPanes()
	if len(watched) == 0 {
		return nil
	}
	isAgentRow := map[string]bool{}
	for _, p := range existing {
		isAgentRow[p.PaneID] = true
	}
	var out []Pane
	for _, id := range watched {
		if isAgentRow[id] {
			continue // it became/was an agent — the agent row already represents it
		}
		f, ok := paneFieldsByID[id]
		if !ok || len(f) < 10 {
			continue // pane gone (marker will be reaped) or malformed
		}
		cwd := f[9]
		project, branch := gitInfo(cwd)
		label := strings.TrimSpace(f[4]) // pane title
		if label == "" {
			label = f[5] // fall back to the current command (bash / vim / …)
		}
		inMode := len(f) >= 11 && f[10] == "1"
		out = append(out, Pane{
			PaneID:  id,
			session: f[1],
			window:  f[2],
			pane:    f[3],
			Loc:     fmt.Sprintf("%s:%s.%s", f[1], f[2], f[3]),
			Task:    label,
			Status:  "", // NOT an agent state — a watched plain pane
			source:  "tmux",
			cwd:     cwd,
			project: project,
			branch:  branch,
			inMode:  inMode,
			Watched: true,
		})
	}
	return out
}

// nativePanes turns the native-session store into `source:"native"` radar rows —
// sensed agents running outside tmux (no pane, so no focus/jump/send). A native
// session that's ALSO live in tmux (its session id matches a tmux pane's resume
// record — e.g. it was adopted) is suppressed; the tmux row wins.
func nativePanes(tmuxPanes []Pane, profiles []agentProfile, now int64) []Pane {
	recs := native.Live(now)
	if len(recs) == 0 {
		return nil
	}
	inTmux := map[string]bool{}
	for _, p := range tmuxPanes {
		if rec, ok := resume.Load(p.Loc); ok && rec.SessionID != "" {
			inTmux[rec.SessionID] = true
		}
	}
	var out []Pane
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
		out = append(out, Pane{
			Agent: name, Status: r.State, source: "native",
			cwd: r.Cwd, role: roleForCwd(r.Cwd),
			project: project, branch: branch, icon: icon,
			activityAt: r.UpdatedAt, Since: since,
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
func sortPanes(panes []Pane) {
	sort.SliceStable(panes, func(i, j int) bool {
		// Watched (non-agent) rows always sort BELOW every agent row — they're a
		// secondary, opt-in presence, never competing with "who needs you".
		if panes[i].Watched != panes[j].Watched {
			return !panes[i].Watched
		}
		if ri, rj := statusRank(panes[i].Status), statusRank(panes[j].Status); ri != rj {
			return ri < rj
		}
		if panes[i].Status == "idle" && panes[j].Status == "idle" && panes[i].Since != panes[j].Since {
			return panes[i].Since > panes[j].Since
		}
		return panes[i].Loc < panes[j].Loc
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
	mt := FileMtime(state.WaitingPath(id))
	return mt > 0 && activityAt > mt+waitStaleGrace
}

// resolveWaiting decides a pane's DISPLAYED status from its raw (title/frame) status
// and its hook waiting mark, returning the resolved status and whether the caller
// should delete the mark. "Waiting is HOOK-DRIVEN": a fresh mark is authoritative.
//
//   - Fresh mark (hasMark && !stale): the pane is blocked on you — show "waiting"
//     even if its title read "working" (a spinner TITLE freezes on its last frame
//     while the agent is blocked on a prompt, so it can't be trusted). The ONE
//     exception is a pane that is DEMONSTRABLY live-working (screen changing / CPU
//     burning): the user just answered and the approved tool is running, so show
//     "working" and let the agent's own PostToolUse→Resumed hook (or staleness)
//     clear the mark — never clear it here (the first poll after a prompt appears
//     also reads live-working, and clearing then drops the mark before the approval
//     card shows). liveWorking is a thunk so the frame/CPU sample is taken only when
//     it can matter (a working title over a fresh mark).
//   - Stale mark (hasMark && stale): a genuine orphan — a resumed turn whose Resumed
//     hook was missed, or a mark inherited when a tmux restart reused this pane id
//     (the "waiting 9d" bug). Keep the raw status and tell the caller to clear it.
//   - No mark: keep the raw status untouched. "Waiting" is never inferred from screen
//     output (a numbered "1. … 2. …" list in an agent's own prose must not pop a
//     bogus approval card) — it belongs to the hook, not the terminal.
func resolveWaiting(status string, hasMark, stale bool, liveWorking func() bool) (out string, clearMark bool) {
	switch {
	case hasMark && !stale:
		if status == "working" && liveWorking() {
			return status, false
		}
		return "waiting", false
	case hasMark:
		return status, true
	default:
		return status, false
	}
}

// roleForCwd marks the supervisor (中控) session: a pane whose working dir is the
// hq home (see `gtmux hq`) carries role:"supervisor" so surfaces can pin or badge
// it. Cwd-keyed on purpose — robust to tmux session renames — and symlink-normalized
// (tmux reports physical paths). This is only the FIRST pass: applyRolePrecedence
// then withdraws the role from every cwd-matched pane whenever a `gtmux hq`-stamped
// pane exists, so a worker parked in the HQ home never impersonates the supervisor.
func roleForCwd(cwd string) string {
	if hqpane.SameDir(cwd, state.HQHome()) {
		return "supervisor"
	}
	return ""
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

// AgentsJSONBytes marshals the live agents into the stable `gtmux agents --json`
// array (no colors, no screen-scraping). Shared by the CLI command and the
// remote server (internal/server) so both speak one byte-identical contract.
func AgentsJSONBytes() ([]byte, error) {
	panes := GatherAgents()
	out := make([]agentJSON, 0, len(panes))
	for _, p := range panes {
		src := p.source
		if src == "" {
			src = "tmux"
		}
		out = append(out, agentJSON{
			PaneID: p.PaneID, Session: p.session, Window: p.window, Pane: p.pane,
			Loc: p.Loc, Agent: p.Agent, Status: p.Status, Task: p.Task,
			Latest: p.Latest, Activity: p.Activity,
			Source: src, Role: p.role, Project: p.project, Branch: p.branch, Terminal: p.terminal, Tab: p.tab,
			ActivityAt: p.activityAt, Since: p.Since, Icon: p.icon,
			SessionID: p.sessionID, Adoptable: p.adoptable,
			Error: p.Errored, ErrorText: p.ErrorText,
			Bg: p.Bg, BgCount: p.BgCount, BgText: p.BgText,
			UsageWarn: p.usageWarn,
			InMode:    p.inMode,
			Watched:   p.Watched,
		})
	}
	return json.MarshalIndent(out, "", "  ")
}
