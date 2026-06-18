package terminal

import (
	"os/exec"
	"strings"
)

// osa runs an AppleScript and returns trimmed stdout. Shared by the AppleScript
// drivers in this package.
func osa(script string) (string, error) {
	out, err := exec.Command("/usr/bin/osascript", "-e", script).Output()
	return strings.TrimSpace(string(out)), err
}

// aplQuote escapes a string for an AppleScript "..." literal.
func aplQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// shellQuote single-quotes a string for the shell (handles spaces etc.).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// isViewing reports whether the frontmost app is one of procNames AND its front
// window title matches the tmux session (== or starts with "<session> — ", per
// set-titles-string '#S — #W'). Terminal-agnostic (System Events); drivers use it
// to suppress a notification you don't need. Best-effort: any error → false.
func isViewing(session string, procNames ...string) bool {
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
	out, err := osa(script)
	if err != nil {
		return false
	}
	parts := strings.SplitN(out, "\n", 2)
	if len(parts) != 2 {
		return false
	}
	proc := strings.ToLower(strings.TrimSpace(parts[0]))
	title := strings.TrimSpace(parts[1])
	matched := false
	for _, p := range procNames {
		if proc == strings.ToLower(p) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	return title == session || strings.HasPrefix(title, session+" — ")
}
