// Package ghostty drives the Ghostty terminal on macOS via AppleScript:
// bringing the tab for a tmux session to the front, and spawning one tab per
// session. All control flows through `osascript`, so callers stay platform-free.
package ghostty

import (
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Driver adapts this package to internal/terminal.Terminal, so Ghostty is one of
// several selectable terminal backends. Methods delegate to the package funcs
// (kept so the native FocusTerminalTab path and internals stay put).
type Driver struct{}

func (Driver) Name() string                            { return "Ghostty" }
func (Driver) FocusTab(session string) (string, error) { return FocusTab(session) }
func (Driver) IsViewing(session string) bool           { return IsViewing(session) }
func (Driver) OpenWindow(command string) (string, error) {
	return OpenWindow(command)
}
func (Driver) SpawnTabs(sessions []string, dryRun bool) (string, error) {
	return SpawnTabs(sessions, dryRun)
}
func (Driver) TabOrder() []string { return TabOrder() }

// TabOrder returns the tmux session names of Ghostty's tabs in order (across
// windows), derived from each tab's title "#S — #W". nil if it can't be read.
func TabOrder() []string {
	out, err := osascript(`tell application "Ghostty"
  set txt to ""
  repeat with w in windows
    repeat with t in tabs of w
      set txt to txt & (name of t) & linefeed
    end repeat
  end repeat
  return txt
end tell`)
	if err != nil {
		return nil
	}
	return SessionsFromTitles(out)
}

// SessionsFromTitles maps newline-separated tab titles ("#S — #W") to their
// session names, in order, de-duplicated, dropping non-matching lines.
func SessionsFromTitles(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		i := strings.Index(t, " — ")
		if i < 0 {
			continue
		}
		name := StripDecoration(t[:i])
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// StripDecoration removes the leading decoration a tab title can carry before the
// session name.
//
// TWO sources put something there, and neither is a mistake to be avoided:
// the TERMINAL does it (Ghostty prefixes a background tab that rang the bell), and
// GTMUX does it (`tab-alert` marks a session with an agent waiting — internal/tabalert).
// So matching a tab to a session must tolerate a prefix by construction rather than know
// about any particular glyph.
//
// It trims leading runes that cannot start a session name — anything that is not a letter
// or a digit. A tmux session name may CONTAIN punctuation, but a leading glyph plus space
// is never part of one.
//
// Shipped v0.51.0 without this on the FOCUS path, which broke exactly the sessions a user
// clicks: `tab-alert` marks only the waiting ones, so only those failed to jump.
func StripDecoration(title string) string {
	return strings.TrimSpace(strings.TrimLeftFunc(title, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}))
}

// TitleMatchesSession reports whether a tab TITLE belongs to `session`, tolerating any
// leading decoration. The separator must match set-titles-string exactly: space, em-dash,
// space.
func TitleMatchesSession(title, session string) bool {
	t := StripDecoration(title)
	return t == session || strings.HasPrefix(t, session+" — ")
}

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
	// The match happens HERE, not inside the AppleScript, so there is ONE matcher and it
	// is testable. The script used to compare `tn starts with "<session> — "` directly,
	// which a decorated title never satisfies — see StripDecoration.
	titles, err := tabTitles()
	if err != nil {
		return "", err
	}
	for _, t := range titles {
		if !TitleMatchesSession(t.name, session) {
			continue
		}
		// Select by the tab's EXACT raw title, through the same `repeat … select tab t`
		// form that has always worked. Addressing it as `tab N of window W` is invalid in
		// Ghostty's dictionary ("Can't get 8 of window 1. Access not allowed. (-1723)"),
		// and the loop variable is the only reference `select` accepts. So the tolerant
		// comparison stays in Go and AppleScript is left with plain string equality.
		return osascript(`tell application "Ghostty"
  repeat with w in windows
    repeat with t in tabs of w
      if (name of t) is "` + Quote(t.name) + `" then
        select tab t
        activate window w
        activate
        return "ok"
      end if
    end repeat
  end repeat
  return "notfound"
end tell`)
	}
	return "notfound", nil
}

// tabTitle is one Ghostty tab, addressable by its 1-based window/tab index.
type tabTitle struct {
	win, tab int
	name     string
}

// tabTitles lists every tab with its position, so a match found in Go can be acted on.
func tabTitles() ([]tabTitle, error) {
	out, err := osascript(`tell application "Ghostty"
  set txt to ""
  repeat with wi from 1 to count of windows
    repeat with ti from 1 to count of tabs of window wi
      set txt to txt & wi & "\t" & ti & "\t" & (name of tab ti of window wi) & linefeed
    end repeat
  end repeat
  return txt
end tell`)
	if err != nil {
		return nil, err
	}
	var rows []tabTitle
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		t, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			continue
		}
		rows = append(rows, tabTitle{win: w, tab: t, name: parts[2]})
	}
	return rows, nil
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
	return TitleMatchesSession(title, session)
}

// FocusTerminalTab brings the tab titled `title` in the terminal app `app` to
// the front — the native-agent jump (DESIGN §7). Ghostty is fully supported
// (tab-title match); other terminals are best-effort (activate the app, since
// per-terminal tab scripting differs). Returns "ok"/"notfound" or "".
func FocusTerminalTab(app, title string) (string, error) {
	if app == "Ghostty" {
		return osascript(ghosttyTabScript(title))
	}
	return osascript(`tell application "` + Quote(app) + `" to activate`)
}

func ghosttyTabScript(title string) string {
	return `tell application "Ghostty"
  repeat with w in windows
    repeat with t in tabs of w
      if name of t is "` + Quote(title) + `" then
        select tab t
        activate window w
        activate
        return "ok"
      end if
    end repeat
  end repeat
  return "notfound"
end tell`
}

// windowScript builds the AppleScript to open a new Ghostty window running a
// shell command (the caller is responsible for shell-quoting within command).
func windowScript(command string) string {
	return `tell application "Ghostty"
  activate
  set cfg to new surface configuration
  set command of cfg to "` + Quote(command) + `"
  new window with configuration cfg
end tell`
}

// OpenWindow opens a new Ghostty window running command (e.g. the gtmux overview
// or the live agents watch). Returns osascript stdout and any error.
func OpenWindow(command string) (string, error) {
	return osascript(windowScript(command))
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
