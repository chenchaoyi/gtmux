package main

import (
	"os"
	"os/exec"
	"strings"
)

// tmuxBin is the resolved path to the tmux binary (may be "" if not found).
// tmux can be absent from PATH when invoked from a bare popup environment, so
// fall back to the common Homebrew locations.
var tmuxBin = resolveTmux()

func resolveTmux() string {
	if p, err := exec.LookPath("tmux"); err == nil {
		return p
	}
	for _, p := range []string{"/opt/homebrew/bin/tmux", "/usr/local/bin/tmux"} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// tmuxRun runs tmux with args and returns trimmed stdout (stderr discarded).
func tmuxRun(args ...string) (string, error) {
	if tmuxBin == "" {
		return "", exec.ErrNotFound
	}
	out, err := exec.Command(tmuxBin, args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

// tmuxOK runs tmux and reports whether it exited 0 (output ignored).
func tmuxOK(args ...string) bool {
	if tmuxBin == "" {
		return false
	}
	return exec.Command(tmuxBin, args...).Run() == nil
}

// tmuxServerUp reports whether a tmux server is running.
func tmuxServerUp() bool { return tmuxBin != "" && tmuxOK("has-session") }

// tmuxLines runs tmux and returns stdout split into non-empty lines.
func tmuxLines(args ...string) []string {
	out, err := tmuxRun(args...)
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// display returns `tmux display-message -p <fmt>` for the given (optional) target.
func display(target, format string) string {
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, format)
	out, _ := tmuxRun(args...)
	return out
}

// inTmux reports whether we are running inside a tmux client.
func inTmux() bool { return os.Getenv("TMUX") != "" }
