package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmdOneshotRun is the hidden in-pane runner behind `gtmux spawn --oneshot`:
// it execs the agent's headless mode (the goal travels as argv — no paste, no
// delivery verification), streams the structured output through to the pane
// (observability: the user can still watch, radar still shows the row via the
// process subtree), and derives the run's lifecycle truth from that stream plus
// the exit code — never from screen classification. Completion is recorded to
// the session-events stream and the state markers, mirroring the hook's Stop /
// StopFailure semantics, as a BACKSTOP: when the run's own hooks already
// recorded it, nothing is double-appended.
//
// usage (plumbing, not a user command): gtmux oneshot-run --agent <key> [--model <m>] -- <goal…>
func cmdOneshotRun(args []string) int {
	agent, model, goalFile := "", "", ""
	var goalParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				i++
				agent = args[i]
			}
		case "--model":
			if i+1 < len(args) {
				i++
				model = args[i]
			}
		case "--goal-file":
			if i+1 < len(args) {
				i++
				goalFile = args[i]
			}
		case "--":
			goalParts = append(goalParts, args[i+1:]...)
			i = len(args)
		default:
			goalParts = append(goalParts, args[i])
		}
	}
	goal := strings.TrimSpace(strings.Join(goalParts, " "))
	if goalFile != "" {
		g, err := readGoalFile(goalFile)
		if err != nil {
			i18n.Sae("gtmux oneshot-run: --goal-file: "+err.Error(), "gtmux oneshot-run: --goal-file: "+err.Error())
			return 2
		}
		goal = g
	}
	spec := driver.For(agent).Headless
	if spec == nil || goal == "" {
		i18n.Sae("gtmux oneshot-run: needs a headless-capable --agent and a goal",
			"gtmux oneshot-run: 需要具备 headless 能力的 --agent 和任务内容")
		return 2
	}

	pane := os.Getenv("TMUX_PANE")
	start := time.Now().Unix()
	if pane != "" {
		_ = state.WriteMarker(state.ActivePath(pane), "") // the turn is live (idempotent with the hook's)
	}

	argv := spec.Args(goal, model)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = scrubHookEnv(os.Environ())
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		i18n.Sae("gtmux oneshot-run: "+err.Error(), "gtmux oneshot-run: "+err.Error())
		finishOneshot(pane, agent, goal, driver.HeadlessOutcome{Summary: err.Error()}, true, start)
		return 1
	}

	var o driver.HeadlessOutcome
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // stream-json lines can be huge
	for sc.Scan() {
		line := sc.Text()
		fmt.Println(line) // print-through: the pane (and the user) sees the stream
		spec.ParseLine(line, &o)
	}
	waitErr := cmd.Wait()

	// The stream is the primary truth; the exit code is the backstop (a stream
	// that never declared a result, or declared success from a run that then
	// died, resolves conservatively to failure).
	failed := o.Failed || waitErr != nil
	finishOneshot(pane, agent, goal, o, failed, start)
	if failed {
		return 1
	}
	return 0
}

// finishOneshot records a one-shot run's completion: the resume record (so the
// digest resolves the session's transcript — the row reads `sense: driver`),
// the state markers, and — when the run's own hooks did not already record it —
// the Stop / StopFailure event, with the stream's result text as the summary.
func finishOneshot(pane, agent, goal string, o driver.HeadlessOutcome, failed bool, start int64) {
	if pane != "" && o.Session != "" {
		if loc := tmux.Display(pane, "#{session_name}:#{window_index}.#{pane_index}"); loc != "" {
			_ = resume.Save(loc, resume.Record{
				Agent: agent, SessionID: o.Session,
				Cwd: cwdOf(pane), UpdatedAt: time.Now().Unix(),
			})
		}
	}
	if pane != "" {
		state.Remove(state.ActivePath(pane))
		state.Remove(state.WaitingPath(pane))
		if failed {
			state.Remove(state.FinishedPath(pane)) // a crash must never read as done
		} else {
			_ = state.WriteMarker(state.FinishedPath(pane), "")
			_ = state.WriteLastFinished(pane)
		}
	}
	if completionRecorded(pane, start) {
		return // the run's own hooks already recorded the completion
	}
	rec := events.Record{
		Ts: time.Now().Unix(), Pane: pane, Agent: oneshotDisplay(agent),
		Summary: radar.Snip(o.Summary, 200),
	}
	if failed {
		rec.Event, rec.State = "StopFailure", "crash"
	} else {
		rec.Event, rec.State = "Stop", "idle"
	}
	events.Append(rec)
}

// completionRecorded reports whether a terminal event for the pane already sits
// on the stream since the run started — the dedup that makes the runner's
// completion record a backstop rather than a duplicate of the hook's.
func completionRecorded(pane string, start int64) bool {
	if pane == "" {
		return false
	}
	now := time.Now().Unix()
	win := now - start + 2
	if win < 1 {
		win = 1
	}
	for _, r := range events.Read(win, now) {
		if r.Pane == pane && r.Ts >= start && (r.Event == "Stop" || r.Event == "StopFailure") {
			return true
		}
	}
	return false
}

// scrubHookEnv drops the variables that would make the one-shot run believe it
// is nested inside another agent session and recursively trigger hooks
// (multiplexer-research ⭐B: clear CLAUDE_CODE_*/CMUX_* before a programmatic
// agent invocation). Everything else passes through — the proxy env the spawn
// wrapped around the runner must reach the agent.
func scrubHookEnv(env []string) []string {
	out := env[:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name == "CLAUDECODE" ||
			strings.HasPrefix(name, "CLAUDE_CODE_") || strings.HasPrefix(name, "CMUX_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// oneshotDisplay renders the event record's agent name consistently with the
// hook's display names for the headless-capable agents.
func oneshotDisplay(agent string) string {
	switch agent {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	}
	return agent
}

// cwdOf reads a pane's current path (best-effort, "" outside tmux).
func cwdOf(pane string) string {
	return tmux.Display(pane, "#{pane_current_path}")
}

// oneshotLaunch types the runner command into a pane's shell, proxy-wrapped by
// construction like any spawn launch.
//
// The goal reaches the runner as FILE CONTENT whenever it is more than one short line.
// The inline form shell-quotes AND whitespace-collapses the goal — it is typed as one
// line into a shell — so a multi-line instruction silently lost its structure, and a long
// one made an unreadable command line. A short single-line goal keeps the inline form
// because seeing the actual command in the pane is worth something.
func oneshotLaunch(pane, agent, model, goal string) {
	cmd := "gtmux oneshot-run --agent " + agent
	if model != "" {
		cmd += " --model " + shellQuote(model)
	}
	if path, err := stageGoalFile(goal); err == nil {
		cmd += " --goal-file " + shellQuote(path)
	} else {
		cmd += " -- " + shellQuote(strings.Join(strings.Fields(goal), " "))
	}
	_ = tmux.SendText(pane, agentenv.Wrap(cmd), true)
}

// stageGoalFile writes a one-shot goal to a private file for the runner to read, so the
// bytes never become a shell word. A short single-line goal is refused (err) so the
// caller keeps the readable inline form.
func stageGoalFile(goal string) (string, error) {
	if !strings.Contains(goal, "\n") && len(goal) <= oneshotInlineMax {
		return "", errInlineGoal
	}
	dir := filepath.Join(state.Dir(), "dispatch", "goals")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "goal-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(goal); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// oneshotInlineMax is how long a single-line goal may be before it is staged to a file
// rather than typed onto the launch line.
const oneshotInlineMax = 200

var errInlineGoal = errors.New("goal is short enough to pass inline")

// readGoalFile reads a staged goal and removes the file — the runner is its only
// consumer, and a goal left on disk is a stale copy of an instruction.
func readGoalFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	_ = os.Remove(path)
	return dispatch.TrimPayload(string(b)), nil
}

// shellQuote single-quotes s for a POSIX shell, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
