package terminal

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// iterm2 drives iTerm2 via AppleScript, mirroring the Ghostty driver. A session's
// tab is matched by its title "#S — #W" (tmux set-titles), exposed as an iTerm2
// session's `name` (which iTerm2 may suffix with " (tmux)" — the prefix-match
// absorbs it).
//
// Two name gotchas, both verified on a real iTerm2: the AppleScript target is
// "iTerm" — NOT "iTerm2", which resolves to the bundle but loads no scripting
// dictionary, so every command errors — while the macOS *process* name is
// "iTerm2" (so IsViewing keys on that).
type iterm2 struct{}

func (iterm2) Name() string { return "iTerm2" }

func (iterm2) FocusTab(session string) (string, error) {
	s := aplQuote(session)
	return osa(`tell application "iTerm"
  repeat with w in windows
    repeat with t in tabs of w
      repeat with ss in sessions of t
        set nm to name of ss
        if nm is "` + s + `" or nm starts with "` + s + ` — " then
          select t
          select w
          activate
          return "ok"
        end if
      end repeat
    end repeat
  end repeat
  return "notfound"
end tell`)
}

// IsViewing can't use the shared System Events path: iTerm2 leaves the AX
// window title empty, so that check never matches. Instead ask iTerm directly
// whether it's frontmost and what its current session is named (the tmux title,
// possibly suffixed " (tmux)" — prefix-match absorbs it).
func (iterm2) IsViewing(session string) bool {
	out, err := osa(`tell application "iTerm"
  if it is not frontmost then return ""
  tell current session of current window to return name
end tell`)
	if err != nil {
		return false
	}
	out = strings.TrimSpace(out)
	return out == session || strings.HasPrefix(out, session+" — ")
}

func (iterm2) OpenWindow(command string) (string, error) {
	return osa(`tell application "iTerm"
  activate
  create window with default profile command "` + aplQuote(command) + `"
end tell`)
}

func (iterm2) SpawnTabs(sessions []string, dryRun bool) (string, error) {
	var b strings.Builder
	b.WriteString("tell application \"iTerm\"\n  activate\n")
	for _, s := range sessions {
		// Absolute tmux path: spawned commands don't inherit the shell PATH.
		// ShellQuote the name so sessions with spaces attach correctly.
		q := aplQuote(tmux.Bin + " attach -t " + shellQuote(s))
		b.WriteString("  if (count of windows) is 0 then\n")
		b.WriteString("    create window with default profile command \"" + q + "\"\n")
		b.WriteString("  else\n")
		b.WriteString("    tell current window to create tab with default profile command \"" + q + "\"\n")
		b.WriteString("  end if\n")
	}
	b.WriteString("end tell")
	script := b.String()
	if dryRun {
		return script, nil
	}
	_, err := osa(script)
	return script, err
}
