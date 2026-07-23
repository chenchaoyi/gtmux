package app

import (
	"encoding/json"
	"fmt"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/terminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// spawnJSON is the `gtmux spawn --json` contract.
type spawnJSON struct {
	TaskID string `json:"task_id,omitempty"`
	PaneID string `json:"pane_id"`
	// Loc is the LIVE tmux locator session:window.pane — the window's tmux number,
	// recomputed each read so it stays correct under renumber-windows (never baked
	// into the name). Title is the window/pane purpose slug. Together they are the
	// standard handle for a spawned window: "<loc> (%pane) · <title>".
	Loc       string `json:"loc,omitempty"`
	Title     string `json:"title,omitempty"`
	Session   string `json:"session"`
	Delivered bool   `json:"delivered"`
	State     string `json:"state"`
	Evidence  string `json:"evidence,omitempty"`
}

// cmdSpawn implements `gtmux spawn` — verified programmatic dispatch. It launches a
// coding agent (new session / reuse --pane / --worktree), through the network proxy
// by construction, waits for it to come up, delivers the task with land-verification,
// and records the dispatch in the ledger. See openspec agent-dispatch.
func cmdSpawn(args []string) int {
	var (
		paneFlag, worktree, model, agent, cwd, title string
		noOpen, headless, force, asJSON              bool
		timeout                                      time.Duration
		goalParts                                    []string
	)
	agent = "claude"
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "-h" || a == "--help":
			return spawnUsage()
		case a == "--pane":
			paneFlag = next()
		case a == "--worktree" || a == "--wt":
			worktree = next()
		case a == "--model" || a == "-m":
			model = next()
		case a == "--agent":
			agent = next()
		case a == "--cwd":
			cwd = next()
		case a == "--title":
			title = next()
		case a == "--no-open":
			noOpen = true
		case a == "--headless":
			// Background heavy work (B/B2): no terminal tab pops, the window is marked
			// background — but the pane still exists, so it stays proxied, land-verified,
			// tracked, and reapable like any dispatch.
			headless, noOpen = true, true
		case a == "--force":
			force = true
		case a == "--json":
			asJSON = true
		case a == "--timeout":
			if d, err := time.ParseDuration(next()); err == nil {
				timeout = d
			}
		case strings.HasPrefix(a, "--"):
			i18n.Sae("gtmux spawn: unknown option '"+a+"'", "gtmux spawn: 未知选项 '"+a+"'")
			return 2
		default:
			goalParts = append(goalParts, a)
		}
	}
	goal := strings.TrimSpace(strings.Join(goalParts, " "))
	if goal == "" {
		return spawnUsage()
	}
	if tmux.Bin == "" {
		i18n.Sae("tmux not installed (brew install tmux)", "未安装 tmux（brew install tmux）")
		return 1
	}

	tune := dispatch.LoadTuning()
	if timeout > 0 {
		tune.DeliverTimeout = int64(timeout.Seconds())
	}

	// Pre-flight (advisory — warns, never blocks). Silenced in --json mode.
	if !asJSON {
		spawnPreflight(model, cwd, goal)
	}

	// Target a pane: reuse --pane, or create a fresh session (optionally in a worktree).
	pane, session, ownSession, wtPath, branch, rc := spawnTarget(paneFlag, worktree, cwd, goal, agent, model, title, noOpen, headless, asJSON)
	if rc != 0 {
		return rc
	}

	// Delivery is a four-state handshake: launched → ready → content-verified →
	// submitted. WaitAgentReady is the READY gate — it blocks until the composer is
	// input-ready and settled (no startup/trust gate, no boot banner, two stable
	// captures), NOT merely until the agent process is up. Pasting a long goal before
	// that truncates it and swallows the Enter. On timeout we FAIL with the pane's
	// capture as evidence and never paste into a not-ready pane.
	if !dispatchbridge.WaitAgentReady(pane, agent, time.Duration(tune.ReadyTimeout)*time.Second) {
		return spawnFail(asJSON, "", pane, session, dispatch.Result{
			State:    dispatch.StateFailed,
			Evidence: "agent composer not ready within the ready timeout\n" + tmux.CaptureFull(pane)})
	}

	// content-verified + submitted: Deliver pastes atomically (bracketed paste-buffer),
	// confirms the FULL goal (head AND tail) holds before Enter, and re-confirms a
	// swallowed Enter without a blind re-paste. Reused as-is (send-submit-reliability);
	// it now runs against a READY composer, so a "fragment" verdict is a real drop, not
	// a mid-boot repaint.
	res := dispatch.Deliver(dispatchbridge.DispatchIO(pane), dispatchbridge.DeliverOpts(pane, agent, force, tune), goal)

	// HQ awaits this dispatch's completion (done-wake-keyed-on-awaited): mark the pane so
	// its next `done` wakes HQ immediately even if the pane is attended.
	if res.Delivered {
		dispatch.MarkAwaited(pane)
	}

	// Record the dispatch (even on failure, so a created session/worktree is reclaimable).
	taskID := ""
	if ownSession || wtPath != "" || res.Delivered {
		taskID = dispatch.NewID(time.Now().UnixNano())
		_ = dispatch.AddTask(dispatch.Task{
			ID: taskID, Pane: pane, Session: session, Agent: agent, Model: model,
			Cwd: cwd, Worktree: wtPath, Branch: branch, Goal: radar.Snip(goal, 200),
			CreatedAt: time.Now().Unix(), Delivered: res.Delivered, OwnSession: ownSession,
			Source: dispatch.SourceHQDispatched,
		})
	}

	return spawnReport(asJSON, taskID, pane, session, res)
}

// spawnTarget resolves the destination pane, creating a session/worktree as needed
// and launching the agent through the proxy. Returns the pane, session, whether we
// created the session, the worktree path/branch, and a non-zero rc on failure.
func spawnTarget(paneFlag, worktree, cwd, goal, agent, model, title string, noOpen, headless, asJSON bool) (pane, session string, ownSession bool, wtPath, branch string, rc int) {
	// Reuse an existing pane.
	if paneFlag != "" {
		if tmux.Display(paneFlag, "#{pane_id}") == "" {
			i18n.Sae("gtmux spawn: pane "+paneFlag+" not found", "gtmux spawn: 找不到 pane "+paneFlag)
			return "", "", false, "", "", 1
		}
		pane = tmux.Display(paneFlag, "#{pane_id}")
		session = tmux.Display(paneFlag, "#{session_name}")
		// If the pane already runs an agent, deliver into it (skip launch); else launch.
		if dispatchbridge.ShellCommands[tmux.Display(pane, "#{pane_current_command}")] {
			launchAgent(pane, agent, model)
		}
		nameDispatchWindow(pane, spawnSlug(title, "", goal), headless) // task-named for a readable fleet
		return pane, session, false, "", "", 0
	}

	// Create a worktree if requested.
	runDir := cwd
	if worktree != "" {
		p, b, err := dispatch.AddWorktree(cwd, worktree)
		if err != nil {
			i18n.Sae("gtmux spawn: worktree: "+err.Error(), "gtmux spawn: worktree 失败："+err.Error())
			return "", "", false, "", "", 1
		}
		wtPath, branch, runDir = p, b, p
		if !asJSON {
			i18n.Say("• worktree "+p+" ("+b+")", "• 已建 worktree "+p+"（"+b+"）")
		}
	}

	// Create a detached session (named from the branch/goal), optionally in runDir.
	name := uniqueSessionName(spawnSessionName(title, branch, goal), sessionExists)
	create := newSessionArgs(name)
	if runDir != "" {
		create = append(create, "-c", runDir)
	}
	created, err := tmux.Run(create...)
	if err != nil || created == "" {
		// Name collision / bad name → let tmux auto-name.
		auto := []string{"new-session", "-d", "-P", "-F", "#{session_name}"}
		if runDir != "" {
			auto = append(auto, "-c", runDir)
		}
		created, err = tmux.Run(auto...)
	}
	if err != nil || created == "" {
		i18n.Sae("gtmux spawn: failed to create a session", "gtmux spawn: 创建 session 失败")
		return "", "", false, "", "", 1
	}
	pane = tmux.Display(created, "#{pane_id}")
	launchAgent(pane, agent, model)
	nameDispatchWindow(pane, spawnSlug(title, branch, goal), headless) // task-named for a readable fleet

	// Open an UNFOCUSED terminal tab (never steal focus) unless --no-open.
	if !noOpen && runtime.GOOS == "darwin" {
		term := terminal.Active()
		_, _ = term.SpawnTabs([]string{created}, false)
	}
	return pane, created, true, wtPath, branch, 0
}

// nameDispatchWindow names the dispatch's window + pane after the task slug so a glance
// at tmux reads what the fleet is doing (charter C). It pins the window name (turns OFF
// automatic-rename, which would otherwise track the running command) and sets the pane
// title. Best-effort — a naming failure never fails the dispatch.
func nameDispatchWindow(pane, slug string, headless bool) {
	if slug == "" || pane == "" {
		return
	}
	_, _ = tmux.Run("set-window-option", "-t", pane, "automatic-rename", "off")
	_, _ = tmux.Run("rename-window", "-t", pane, windowName(slug, headless))
	_, _ = tmux.Run("select-pane", "-t", pane, "-T", slug)
}

// headlessMarker prefixes a background (`--headless`) dispatch's window name so a glance
// at tmux distinguishes windows the user should WATCH from background work (charter C).
const headlessMarker = "⌁ "

// windowName is the window title for a dispatch: the task slug, prefixed with the
// background marker for a headless dispatch.
func windowName(slug string, headless bool) string {
	if headless && slug != "" {
		return headlessMarker + slug
	}
	return slug
}

// spawnSlug derives a short, tmux-friendly task slug for the window/pane title: an
// explicit --title, else the worktree branch's leaf (feat/menubar-width → menubar-width),
// else a normalized head of the goal.
func spawnSlug(title, branch, goal string) string {
	if s := slugify(title); s != "" {
		return s
	}
	if branch != "" {
		if s := slugify(path.Base(branch)); s != "" {
			return s
		}
	}
	return slugify(firstWords(goal, 4))
}

// slugify lowercases, collapses any run of non-alphanumeric characters to a single '-',
// trims stray '-', and caps the length — a safe, readable tmux window name.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 24 {
		out = strings.Trim(out[:24], "-")
	}
	return out
}

// firstWords returns the first n whitespace-separated words of s.
func firstWords(s string, n int) string {
	f := strings.Fields(s)
	if len(f) > n {
		f = f[:n]
	}
	return strings.Join(f, " ")
}

// launchAgent types the proxy-wrapped agent launch command into a pane's shell —
// the proxy is applied BY CONSTRUCTION (fixes incident ①).
func launchAgent(pane, agent, model string) {
	cmd := agent
	if model != "" {
		cmd += " --model " + model
	}
	_ = tmux.SendText(pane, agentenv.Wrap(cmd), true)
}

// spawnPreflight prints advisory checks: which proxy the launch will apply, the
// machine resource watermark, a model suggestion when the window is tight, and the
// pitfalls/workflows knowledge that matches this dispatch (the consult half-loop's tool
// layer — surfacing captured knowledge at the moment work starts).
func spawnPreflight(model, cwd, goal string) {
	if u := agentenv.Active(); u != "" {
		i18n.Say("• proxy: "+u, "• 代理："+u)
	} else {
		i18n.Say("• proxy: none (direct) — if the agent 403s, a proxy may be needed",
			"• 代理：无（直连）—— 若 agent 报 403，可能需要代理")
	}
	radar.PreflightResource()
	if model == "" {
		if r, ok := limits.Load(); ok && r.Warn != "" {
			i18n.Say("• subscription tight ("+r.Warn+") — consider --model sonnet/haiku",
				"• 订阅额度紧张（"+r.Warn+"）—— 可考虑 --model sonnet/haiku")
		}
	}
	if kb := hq.MatchKnowledge(cwd, goal); kb != "" {
		fmt.Println(kb)
	}
}

// spawnSessionName derives a tmux session name from the branch, else the goal head.
func spawnSessionName(title, branch, goal string) string {
	// TITLE FIRST. --title is the caller stating what this session is FOR, and it was
	// reaching only the window name — the session kept a name derived from the goal, on
	// every path, not just when delivery failed. That is where names like
	// `你是一次性-worker(不是-HQ,不要` came from: a goal's opening words used verbatim.
	src := firstNonEmpty(title, branch, goal)
	var b strings.Builder
	for _, r := range src {
		switch {
		case r == '.' || r == ':' || r == '/' || unicode.IsSpace(r):
			b.WriteRune('-') // tmux treats . and : as target separators
		case unicode.IsPunct(r) && r != '-' && r != '_':
			// Brackets, commas and friends survived the old mapping and made session
			// names that read as noise and are awkward to type as a tmux target.
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	name := collapseDashes(strings.Trim(b.String(), "-"))
	// Truncate by RUNE, not byte. The old cut was `name[:40]`, which slices mid-character
	// on any multi-byte script and yields mojibake — latent for any goal over 40 bytes,
	// which two Chinese words already are.
	if r := []rune(name); len(r) > spawnNameMaxRunes {
		name = strings.Trim(string(r[:spawnNameMaxRunes]), "-")
	}
	return name
}

// sessionExists reports whether a tmux session of that name is live.
func sessionExists(name string) bool {
	return tmux.OK("has-session", "-t", "="+name)
}

// spawnNameMaxRunes bounds a session name so it stays readable in a status line and
// typable as a tmux target.
const spawnNameMaxRunes = 24

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// collapseDashes squeezes runs of '-' left by sanitizing adjacent punctuation.
func collapseDashes(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// uniqueSessionName appends a numeric suffix until the name is free.
//
// Without this a name collision fell through to tmux's own auto-naming, which numbers
// sessions — that is where a session called `12` came from. A collision is ordinary (two
// dispatches for the same goal), and answering it with a bare number throws away the name
// the caller asked for instead of adjusting it.
func uniqueSessionName(name string, exists func(string) bool) string {
	if name == "" {
		return ""
	}
	if !exists(name) {
		return name
	}
	for i := 2; i < 100; i++ {
		cand := name + "-" + strconv.Itoa(i)
		if !exists(cand) {
			return cand
		}
	}
	return ""
}

// spawnHandle formats the STANDARD handle for a spawned window: "<loc> (%pane) · <title>"
// — the live tmux number (loc, so you can jump by number, correct under
// renumber-windows) plus the concise purpose title. Degrades gracefully when loc/title
// are unknown. Pure, so the format is unit-testable.
func spawnHandle(loc, pane, title string) string {
	h := pane
	if loc != "" {
		h = loc + " (" + pane + ")"
	}
	if title != "" {
		h += " · " + title
	}
	return h
}

// spawnLocator reads the spawned pane's LIVE standard handle: its tmux locator
// (session:window.pane — the window number, live so renumber-windows never staled it)
// and its purpose title (the pane title = the slug we set). Best-effort ("" on failure).
func spawnLocator(pane string) (loc, title string) {
	if pane == "" {
		return "", ""
	}
	loc = tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}")
	title = strings.TrimSpace(tmux.Display(pane, "#{pane_title}"))
	return loc, title
}

// spawnReport prints the outcome and returns the exit code (non-zero unless landed).
func spawnReport(asJSON bool, taskID, pane, session string, res dispatch.Result) int {
	loc, title := spawnLocator(pane)
	handle := spawnHandle(loc, pane, title)
	if asJSON {
		b, _ := json.MarshalIndent(spawnJSON{
			TaskID: taskID, PaneID: pane, Loc: loc, Title: title, Session: session,
			Delivered: res.Delivered, State: string(res.State), Evidence: res.Evidence,
		}, "", "  ")
		fmt.Println(string(b))
	} else {
		switch res.State {
		case dispatch.StateLanded:
			i18n.Say("✓ dispatched → "+handle, "✓ 已派活 → "+handle)
		case dispatch.StateQueued:
			i18n.Say("• queued → "+handle+" — runs after the current turn",
				"• 已排队 → "+handle+" —— 当前这轮结束后执行")
		case dispatch.StateRefusedDup:
			// The handle belongs on the FAILURE lines most of all: a bare "%37" is the
			// one identifier you can't act on — you can't jump to it, and it says nothing
			// about which session or what the window was for. A failed dispatch leaves a
			// live session behind, so naming it is how you find it.
			i18n.Sae("✗ refused → "+handle+": identical payload re-sent within the window (use --force)",
				"✗ 拒发 → "+handle+"：时间窗内重复相同内容（要重发用 --force）")
		default:
			i18n.Sae("✗ NOT delivered → "+handle+" — evidence:\n"+res.Evidence,
				"✗ 未送达 → "+handle+" —— 证据：\n"+res.Evidence)
		}
	}
	if res.Delivered {
		return 0
	}
	return 1
}

// spawnFail is spawnReport for an early failure with no ledger entry.
func spawnFail(asJSON bool, taskID, pane, session string, res dispatch.Result) int {
	return spawnReport(asJSON, taskID, pane, session, res)
}

func spawnUsage() int {
	i18n.Sae("usage: gtmux spawn [--pane <id>] [--worktree <branch>] [--title <slug>] [--model <m>] [--agent <cmd>] [--cwd <dir>] [--headless] [--no-open] [--force] [--json] <goal…>",
		"用法：gtmux spawn [--pane <id>] [--worktree <分支>] [--title <名>] [--model <模型>] [--agent <命令>] [--cwd <目录>] [--headless] [--no-open] [--force] [--json] <任务…>")
	return 2
}
