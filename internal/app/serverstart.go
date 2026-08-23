package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// fromAppFlag is how a caller says "I am an application" — the one fact that decides
// whether the boot server needs an identity of its own. See startBootServer.
const fromAppFlag = "--from-app"

// startBootServer creates the tmux server that a restore will restore INTO, and returns
// whether it succeeded.
//
// The question this answers is not "how do I start tmux" but "whose name will macOS put
// on everything that happens inside it".
//
// macOS attributes a file access to the RESPONSIBLE process — the application at the root
// of the process tree — and fixes that attribution when a process is created. It survives
// being re-parented to launchd, which is why nothing in `ps` reveals it: the tmux server
// shows ppid=1 and looks unowned. Measured live on 2026-08-23: an agent's `ls` of another
// app's container, four processes deep inside a server that `gtmux restore` had created
// from the menu-bar app, prompted as "Gtmux.app would like to access data from other
// apps". gtmux was doing nothing; the user's own agent was clearing disk space. The name
// on the prompt was simply wrong, and a background app that repeatedly asks for other
// apps' data does not look like an app with a naming bug — it looks like one helping
// itself.
//
// launchd hands out a fresh identity, so a server it starts answers as tmux. Measured
// both ways, same read:
//
//	started directly   →  responsible = GtmuxBar
//	started by launchd →  responsible = tmux
//
// (setsid does nothing here: responsibility is neither the process group nor the parent,
// and survives both.)
//
// This detour is taken ONLY for a caller that says it is an application. A restore run
// from a terminal is already attributed to that terminal — which IS the application the
// user launched, so its name on a prompt is the honest answer, and there is nothing to
// fix. gtmux cannot tell the two apart by itself (reading one's own responsible process
// needs privilege), so the app declares it.
//
// Every failure falls back to the ordinary path. Getting the working set back after a
// reboot is the point of this command; a restore that returns everything under the wrong
// name is a cosmetic defect, and one that returns nothing is a real one.
func startBootServer(session string, fromApp bool) bool {
	if fromApp && runtime.GOOS == "darwin" {
		if startBootServerViaLaunchd(session) {
			restoreLogf("ensureServer: boot server started via launchd (own identity)")
			return true
		}
		restoreLogf("ensureServer: launchd start failed — falling back to a direct start")
	}
	return tmux.OK("new-session", "-d", "-s", session)
}

// startBootServerViaLaunchd asks launchd to start the server, then waits for it to come
// up — `launchctl submit` returns as soon as the job is accepted, not when tmux is
// listening, and the caller's very next act is to talk to that server.
//
// The environment is passed EXPLICITLY and this is not a detail: a launchd-started
// process inherits none of the user's. Measured for this change, a launchd-started tmux
// hands its panes `PATH=/usr/bin:/bin:/usr/sbin:/sbin` and no LANG at all — no Homebrew,
// so no tmux, no git, no agent binaries; and no language, which is the exact shape of the
// v0.11.3 failure where a launchd-started serve left tmux mangling UTF-8 and the radar
// reported zero agents on a machine full of them.
func startBootServerViaLaunchd(session string) bool {
	if _, err := exec.LookPath("launchctl"); err != nil {
		return false
	}
	label := fmt.Sprintf("com.gtmux.boot.%d", os.Getpid())
	inner := fmt.Sprintf("%s new-session -d -s %s", shellQuote(tmux.Bin), shellQuote(session))
	// exported before the command, so the SERVER holds them and every pane it later
	// creates inherits them.
	script := strings.Join(bootServerEnv(), " ") + " exec " + inner

	if err := exec.Command("launchctl", "submit", "-l", label, "--", "/bin/sh", "-c", script).Run(); err != nil {
		return false
	}
	// launchctl keeps the (now-exited) job registered; drop it so a later restore can
	// reuse the label space and `launchctl list` stays clean.
	defer func() { _ = exec.Command("launchctl", "remove", label).Run() }()
	return waitForServer(bootServerWait)
}

// bootServerWait bounds how long to wait for a launchd-started server to answer. It is
// a local `tmux new-session -d` behind a job submission, so it is up in well under a
// second or something is wrong and the fallback should run.
const bootServerWait = 5 * time.Second

// waitForServer polls until the tmux server answers, or the deadline passes.
func waitForServer(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if tmux.ServerUp() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// bootServerEnv is the environment a launchd-started server must be given by hand. Kept
// small and explicit: the language (so non-ASCII pane titles survive) and a PATH that can
// actually find tmux, git and the agents.
func bootServerEnv() []string {
	var out []string
	for _, k := range []string{"LANG", "LC_ALL", "LC_CTYPE", "PATH", "HOME", "TMUX_TMPDIR"} {
		if v := os.Getenv(k); v != "" {
			out = append(out, k+"="+shellQuote(v))
		}
	}
	// A missing language is the failure that costs the most and shows the least, so
	// never leave it unset — UTF-8 is the right default for every locale gtmux serves.
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" && os.Getenv("LC_CTYPE") == "" {
		out = append(out, "LC_CTYPE=UTF-8")
	}
	return out
}
