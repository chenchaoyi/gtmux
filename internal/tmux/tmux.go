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

// command builds a tmux invocation that always speaks UTF-8. Without this, tmux
// run from an environment with no UTF-8 locale (notably a LaunchAgent — the
// serve/tunnel daemons inherit NO LANG/LC_*) substitutes every non-ASCII byte
// in pane titles / captures with "_". That mangles the ✳/braille agent glyphs
// `classifyAgent` keys off, so the radar silently reports ZERO agents (the
// "phone connected but empty" bug). `-u` forces UTF-8 OUTPUT (titles/captures);
// a UTF-8 LC_CTYPE fixes INPUT so `send-keys -l` of CJK text isn't garbled.
func command(args ...string) *exec.Cmd {
	c := exec.Command(Bin, append([]string{"-u"}, args...)...)
	c.Env = utf8Env()
	return c
}

// utf8Env returns the parent environment, guaranteeing a UTF-8 LC_CTYPE when
// none of LC_ALL/LC_CTYPE/LANG already selects one.
func utf8Env() []string {
	env := os.Environ()
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := strings.ToUpper(os.Getenv(k)); strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8") {
			return env
		}
	}
	return append(env, "LC_CTYPE=en_US.UTF-8")
}

// Run runs tmux with args and returns trimmed stdout (stderr discarded).
func Run(args ...string) (string, error) {
	if Bin == "" {
		return "", exec.ErrNotFound
	}
	out, err := command(args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

// OK runs tmux and reports whether it exited 0 (output ignored).
func OK(args ...string) bool {
	if Bin == "" {
		return false
	}
	return command(args...).Run() == nil
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
// when the agent sets no title spinner. "" on error. PLAIN text (no escapes) so
// frame-diffing stays stable.
func CapturePane(pane string) string {
	out, _ := Run("capture-pane", "-p", "-t", pane)
	return out
}

// CapturePaneColor returns the pane's screen + scrollback WITH ANSI SGR escapes
// (`-e`), so the mobile app can render history in color (MOBILE §4). `-S -2000`
// includes up to 2000 lines of scrollback (bounded for payload/render cost; the
// real depth is also capped by tmux history-limit). Read-only.
func CapturePaneColor(pane string) string {
	out, _ := Run("capture-pane", "-e", "-p", "-S", "-2000", "-t", pane)
	return out
}

// SendText types literal text into a pane (`send-keys -l`, so the text is never
// interpreted as tmux key names), optionally followed by Enter. This is a WRITE.
func SendText(pane, text string, enter bool) error {
	if text != "" {
		if _, err := Run("send-keys", "-t", pane, "-l", text); err != nil {
			return err
		}
	}
	if enter {
		_, err := Run("send-keys", "-t", pane, "Enter")
		return err
	}
	return nil
}

// SendKey sends a single NAMED key (e.g. Enter, C-c, Escape, Tab, Up, Down) to a
// pane. Callers MUST validate `key` against an allowlist — it is a tmux key name.
func SendKey(pane, key string) error {
	_, err := Run("send-keys", "-t", pane, key)
	return err
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
	c := command(args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}
