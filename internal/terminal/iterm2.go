package terminal

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// iterm2 drives iTerm2 via AppleScript, mirroring the Ghostty driver. A session's
// tab is matched by its title "#S — #W" (tmux set-titles), exposed as an iTerm2
// session's `name`.
//
// NOTE: written from the iTerm2 scripting dictionary but NOT runtime-verified on
// the (Ghostty-only) dev machine — verify on iTerm2 before trusting. It only runs
// when the host terminal is detected as iTerm2, so Ghostty users are unaffected.
type iterm2 struct{}

func (iterm2) Name() string { return "iTerm2" }

func (iterm2) FocusTab(session string) (string, error) {
	s := aplQuote(session)
	return osa(`tell application "iTerm2"
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

func (iterm2) IsViewing(session string) bool {
	return isViewing(session, "iTerm2")
}

func (iterm2) OpenWindow(command string) (string, error) {
	return osa(`tell application "iTerm2"
  activate
  create window with default profile command "` + aplQuote(command) + `"
end tell`)
}

func (iterm2) SpawnTabs(sessions []string, dryRun bool) (string, error) {
	var b strings.Builder
	b.WriteString("tell application \"iTerm2\"\n  activate\n")
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
