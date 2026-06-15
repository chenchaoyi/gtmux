// Package ghostty drives the Ghostty terminal on macOS via AppleScript:
// bringing the tab for a tmux session to the front, and spawning one tab per
// session. All control flows through `osascript`, so callers stay platform-free.
package ghostty

import (
	"os/exec"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// osascript runs an AppleScript and returns trimmed stdout.
func osascript(script string) (string, error) {
	out, err := exec.Command("/usr/bin/osascript", "-e", script).Output()
	return strings.TrimSpace(string(out)), err
}

// Quote escapes a string for use inside an AppleScript "..." literal.
func Quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// ShellQuote single-quotes a string for the shell (handles spaces etc.).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// FocusTab brings the Ghostty tab whose title is `session` (or starts with
// "session — ", per set-titles-string '#S — #W') to the front.
// Returns "ok", "notfound", or "" with a non-nil error on AppleScript failure.
func FocusTab(session string) (string, error) {
	// The separator must match set-titles-string exactly: space, em-dash, space.
	script := `tell application "Ghostty"
  repeat with w in windows
    repeat with t in tabs of w
      set tn to name of t
      if tn is "` + Quote(session) + `" or tn starts with "` + Quote(session) + ` — " then
        select tab t
        activate window w
        activate
        return "ok"
      end if
    end repeat
  end repeat
  return "notfound"
end tell`
	return osascript(script)
}

// IsViewing reports whether you are already looking at this session's tab:
// the frontmost macOS app is Ghostty AND its front-window title (which tmux's
// set-titles-string keeps as '#S — #W') equals `session` or starts with
// "session — ". Used to suppress a notification you don't need. Best-effort:
// any AppleScript error returns false (don't suppress).
//
// System Events may report the process name lowercase ("ghostty"), so both are
// accepted. The separator must match set-titles-string exactly: space, em-dash,
// space.
func IsViewing(session string) bool {
	const script = `tell application "System Events"
  set frontProc to first application process whose frontmost is true
  set procName to name of frontProc
  set winTitle to ""
  try
    set winTitle to name of front window of frontProc
  end try
end tell
return procName & "
" & winTitle`
	out, err := osascript(script)
	if err != nil {
		return false
	}
	parts := strings.SplitN(out, "\n", 2)
	if len(parts) != 2 {
		return false
	}
	proc := strings.ToLower(strings.TrimSpace(parts[0]))
	title := strings.TrimSpace(parts[1])
	if proc != "ghostty" {
		return false
	}
	return title == session || strings.HasPrefix(title, session+" — ")
}

// SpawnTabs opens one Ghostty tab per session, each running
// `tmux attach -t <session>`. Returns the generated AppleScript and any error.
// dryRun returns the script without executing it.
func SpawnTabs(sessions []string, dryRun bool) (string, error) {
	var b strings.Builder
	b.WriteString("tell application \"Ghostty\"\n  activate\n")
	for _, s := range sessions {
		// `command` runs instead of a shell, so the tab closes on detach.
		// Absolute tmux path: Ghostty-spawned commands don't inherit shell PATH.
		// ShellQuote the name so session names with spaces attach correctly.
		cmd := tmux.Bin + " attach -t " + ShellQuote(s)
		b.WriteString("  set cfg to new surface configuration\n")
		b.WriteString("  set command of cfg to \"" + Quote(cmd) + "\"\n")
		b.WriteString("  if (count of windows) is 0 then\n")
		b.WriteString("    new window with configuration cfg\n")
		b.WriteString("  else\n")
		b.WriteString("    new tab in front window with configuration cfg\n")
		b.WriteString("  end if\n")
	}
	b.WriteString("end tell")
	script := b.String()
	if dryRun {
		return script, nil
	}
	_, err := osascript(script)
	return script, err
}
