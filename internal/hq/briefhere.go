package hq

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Handing THIS pane to the agent, when `gtmux hq` is running in it.
//
// `gtmux hq --here`, and a revive of a stamped pane the user happens to be sitting in,
// both end with "start the agent in that pane". When the pane is someone else's, gtmux
// types the launch command into it and then watches it come up. When the pane is its OWN,
// neither half works: gtmux holds the terminal, so everything it types is buffered by the
// tty and handed to the agent as stdin the moment the shell starts it, and the pane it
// watches shows gtmux's own shell, not a composer.
//
// On 2026-09-20 that produced the whole failure at once: the ready gate read the dead
// session's leftover composer row as ready, the briefing was pasted into the buffer,
// gtmux reported "not delivered", and when the shell finally ran the agent the buffered
// text arrived as an unsubmitted draft in its input box.
//
// So this path types nothing. It hands the briefing to a detached copy of gtmux that
// watches the pane from outside, and REPLACES this process with the agent, which makes
// the agent the pane's foreground command immediately and leaves no buffered input.

// briefWatchBudget is how long the detached watcher waits for the agent's composer.
// Generous on purpose: nobody is waiting on it, and the wait now spans the shell starting
// the agent as well as the agent's own boot.
const briefWatchBudget = 3 * time.Minute

// briefPaneFlag is the hidden flag that runs the detached half.
const briefPaneFlag = "--brief-pane"

// startsInOwnPane reports whether pane is the pane this command is running in.
func startsInOwnPane(pane string) bool {
	own := ownPane()
	return own != "" && pane == own
}

// startHQHere starts the agent in the pane gtmux is running in: the briefing goes to a
// detached watcher, then this process becomes the agent. rawCmd is the agent command
// (the watcher needs it to recognise the agent); dir is the directory to start it in,
// or "" to stay put.
//
// It returns only when the hand-over could not happen at all, and then it says so and
// names the command, because the one thing it must not do is type into this terminal.
func startHQHere(pane, rawCmd, dir string) int {
	startBriefWatcher(pane, rawCmd)
	shellCmd := hereCommand(rawCmd, dir)
	err := execInShell(shellCmd)
	diag.Did("act.hq.start", pane, diag.Failed, "this terminal could not be handed to the agent",
		"error", err, "how", "here")
	i18n.Sae("gtmux hq: could not hand this terminal to the agent ("+err.Error()+"). Run it yourself:\n  "+shellCmd,
		"gtmux hq: 没能把这个终端交给 agent（"+err.Error()+"）。请自己跑一下：\n  "+shellCmd)
	return 1
}

// hereCommand is what the shell runs: the proxy-wrapped agent command, in dir, with
// `exec` in front of it.
//
// The `exec` is load-bearing. Without it the shell stays as the pane's foreground process
// group leader, which is what tmux reports as the pane's current command — so the ready
// gate would watch for the agent to take the pane over and see the shell forever, and the
// briefing would never be delivered (measured on the probe: `bash`, with the agent
// running inside it). With it, the shell IS the agent from that moment.
func hereCommand(rawCmd, dir string) string {
	cmd := "exec " + agentenv.Wrap(rawCmd)
	if dir != "" {
		cmd = "cd " + shq(dir) + " && " + cmd
	}
	return cmd
}

// execInShell replaces this process with the user's shell running cmd, so the agent —
// not gtmux — owns the terminal from here on.
func execInShell(cmd string) error {
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	return syscall.Exec(sh, []string{filepath.Base(sh), "-c", cmd}, os.Environ())
}

// startBriefWatcher launches the detached half. It is best-effort: without it HQ simply
// starts without its briefing, which is the documented fallback.
func startBriefWatcher(pane, rawCmd string) bool {
	if !hqBriefingEnabled() {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	c := exec.Command(exe, briefWatcherArgs(pane, rawCmd)...)
	c.Dir = ownDir()
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // outlives this process's exec
	if err := c.Start(); err != nil {
		return false
	}
	_ = c.Process.Release()
	return true
}

// briefWatcherArgs is the detached half's argument list.
func briefWatcherArgs(pane, rawCmd string) []string {
	return []string{"hq", briefPaneFlag, pane, "--agent", rawCmd}
}

func ownDir() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}

// briefPaneWorker is the detached half: it waits for the agent to take the pane over and
// its composer to settle, then delivers the briefing through the ordinary verified path,
// draft guard included. It has no terminal to report on, so the outcome goes to the log
// (`gtmux logs --event act.hq.brief`).
func briefPaneWorker(pane, agentCmd string) int {
	if pane == "" || !hqBriefingEnabled() {
		return 0
	}
	tune := dispatch.LoadTuning()
	if !dispatchbridge.WaitAgentReady(pane, agentCmd, briefWatchBudget) {
		blocker, _ := dispatchbridge.ReadyBlocker(pane, agentCmd)
		diag.Did("act.hq.brief", pane, diag.Failed, "HQ's startup briefing never had a composer to land in",
			"error", blocker, "agent", agentCmd)
		return 1
	}
	res := dispatch.Deliver(dispatchbridge.DispatchIO(pane),
		dispatchbridge.DeliverOpts(pane, agentCmd, false, tune), hqBriefingPrompt())
	landed := briefLanded(res)
	diag.Did("act.hq.brief", pane, briefOutcome(landed), "delivered HQ's startup briefing",
		"state", string(res.State), "judgedBy", res.JudgedBy, "how", "detached")
	if !landed {
		return 1
	}
	return 0
}

// briefLanded reads a delivery's result the way the briefing cares about it. A QUEUED
// delivery is a landed one: the agent took the text and runs it after the turn it is in
// (`gtmux send` and `spawn` have always read it that way). Calling it "not delivered"
// told the operator to go type into a pane that was about to brief itself.
func briefLanded(res dispatch.Result) bool {
	return res.Delivered || res.State == dispatch.StateQueued
}

func briefOutcome(landed bool) string {
	if landed {
		return diag.OK
	}
	return diag.Failed
}
