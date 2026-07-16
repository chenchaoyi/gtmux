// Package tmux is a thin wrapper around the tmux CLI: it resolves the binary
// once and exposes the small set of invocations gtmux needs (capture output,
// check exit status, split lines, display-message, run interactively).
package tmux

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
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

// runRaw is Run WITHOUT trimming trailing newlines — for capture-pane, where the
// pane's trailing blank rows must be preserved so a bottom-anchored cursor offset
// (pane_height-1-cursor_y) maps to the right line. Trimming them shifted the
// rendered cursor up by however many blank rows sat below the content.
func runRaw(args ...string) (string, error) {
	if Bin == "" {
		return "", exec.ErrNotFound
	}
	out, err := command(args...).Output()
	return string(out), err
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
	// runRaw (not Run): keep the pane's trailing blank rows so the bottom-anchored
	// text cursor (pane_height-1-cursor_y) lands on the right line in the renderer.
	out, _ := runRaw("capture-pane", "-e", "-p", "-S", "-2000", "-t", pane)
	return out
}

// CaptureFull returns a pane's visible screen PLUS a bounded scrollback margin as
// PLAIN text (no ANSI escapes) — the delivery verifier reads it to locate the input
// box and find a landed message that may have scrolled just above the fold. `-S
// -200` bounds the scrollback (render/parse cost); no `-e`, so box-drawing/text
// parsing stays clean. Read-only.
func CaptureFull(pane string) string {
	out, _ := runRaw("capture-pane", "-p", "-S", "-200", "-t", pane)
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

// pasteSeq disambiguates concurrent paste buffers within one process.
var pasteSeq uint64

// Paste delivers text into a pane WITHOUT interpreting it as keys and WITHOUT an
// auto-Enter: it stages the bytes on a private tmux paste buffer (`load-buffer -`,
// byte-exact from stdin) then `paste-buffer -d` (deletes the buffer afterward).
// This is the delivery path for dispatched task text — `send-keys -l` of a long
// string mid-TUI errored "not in a mode" and was the vector for both the fragment
// and the swallowed-Enter failures. Submission (Enter) is a SEPARATE step
// (SendKey/SendText) so verification can sit between paste and submit and re-send
// Enter on its own. This is a WRITE.
func Paste(pane, text string) error {
	if Bin == "" {
		return exec.ErrNotFound
	}
	buf := "gtmux-dispatch-" + strconv.Itoa(os.Getpid()) + "-" +
		strconv.FormatUint(atomic.AddUint64(&pasteSeq, 1), 10)
	load := command("load-buffer", "-b", buf, "-")
	load.Stdin = strings.NewReader(text)
	if err := load.Run(); err != nil {
		return err
	}
	_, err := Run("paste-buffer", "-d", "-b", buf, "-t", pane)
	return err
}

// InMode reports whether a pane is in a tmux mode (copy-mode / view-mode — i.e. the
// user is scrolling the scrollback). While a pane is in a mode, keys are interpreted
// as mode navigation commands and NEVER reach the running program: `send-keys -l`,
// `paste-buffer`, and Enter are all silently swallowed. Callers that must guarantee
// input lands (dispatch delivery, `gtmux send`, POST /api/send) should ExitCopyMode
// first. Read-only.
func InMode(pane string) bool { return Display(pane, "#{pane_in_mode}") == "1" }

// ExitCopyMode drops a pane out of copy/view-mode (`send-keys -X cancel`) so that
// subsequent input actually reaches the program. It is a no-op (nil) when the pane
// is NOT in a mode — `-X cancel` errors "not in a mode" otherwise, so we gate on
// InMode. This is a WRITE.
func ExitCopyMode(pane string) error {
	if !InMode(pane) {
		return nil
	}
	_, err := Run("send-keys", "-t", pane, "-X", "cancel")
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

// Attended reports whether pane is the FOCUSED pane of an attached tmux client —
// the human is (very likely) looking at it right now. Focused = the pane is active
// in its window AND that window is active in its session AND the session has at
// least one attached client. Used by the HQ wake channel (hq-perception-v2): a
// completion under the user's eyes defers to the summary tick instead of knocking.
func Attended(pane string) bool {
	return attendedFrom(Lines("list-panes", "-a", "-F",
		"#{pane_id}\t#{pane_active}\t#{window_active}\t#{session_attached}"), pane)
}

// attendedFrom is the pure core of Attended (testable without tmux).
func attendedFrom(lines []string, pane string) bool {
	for _, ln := range lines {
		f := strings.Split(ln, "\t")
		if len(f) != 4 || f[0] != pane {
			continue
		}
		return f[1] == "1" && f[2] == "1" && f[3] != "0" && f[3] != ""
	}
	return false
}
