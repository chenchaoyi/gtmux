// Package tmux is a thin wrapper around the tmux CLI: it resolves the binary
// once and exposes the small set of invocations gtmux needs (capture output,
// check exit status, split lines, display-message, run interactively).
package tmux

import (
	"os"
	"os/exec"
	"strings"
)

// Bin is the resolved path to the tmux binary (may be "" if not found).
// tmux can be absent from PATH when invoked from a bare popup environment, so
// fall back to the common Homebrew locations.
var Bin = resolve()

func resolve() string {
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

// Run runs tmux with args and returns trimmed stdout (stderr discarded).
func Run(args ...string) (string, error) {
	if Bin == "" {
		return "", exec.ErrNotFound
	}
	out, err := exec.Command(Bin, args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

// OK runs tmux and reports whether it exited 0 (output ignored).
func OK(args ...string) bool {
	if Bin == "" {
		return false
	}
	return exec.Command(Bin, args...).Run() == nil
}

// ServerUp reports whether a tmux server is running.
func ServerUp() bool { return Bin != "" && OK("has-session") }

// Lines runs tmux and returns stdout split into non-empty lines.
func Lines(args ...string) []string {
	out, err := Run(args...)
	if err != nil || out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// CapturePane returns the visible (current-screen) content of a pane, read-only.
// Used to tell a working agent (its screen animates) from an idle one (static)
// when the agent sets no title spinner. "" on error.
func CapturePane(pane string) string {
	out, _ := Run("capture-pane", "-p", "-t", pane)
	return out
}

// Display returns `tmux display-message -p <fmt>` for the given (optional) target.
func Display(target, format string) string {
	args := []string{"display-message", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, format)
	out, _ := Run(args...)
	return out
}

// InTmux reports whether we are running inside a tmux client.
func InTmux() bool { return os.Getenv("TMUX") != "" }

// RunInteractive runs a tmux subcommand inheriting stdio and returns its exit code.
func RunInteractive(args ...string) int {
	c := exec.Command(Bin, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}
