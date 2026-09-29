package terminal

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmux is a Ghostty-based terminal, but its workspace and terminal controls
// belong to cmux. Its AppleScript dictionary exposes each panel's live title,
// focus, and selected workspace. Never send cmux sessions through Ghostty's
// driver. The CLI socket can reject callers outside cmux (such as the menu bar
// app), so workspace creation uses the AppleScript dictionary too.
type cmux struct{}

func (cmux) Name() string { return "cmux" }

// Read the live panel titles before deciding which panel to focus. Matching in
// Go shares the same decoration handling as Ghostty (bell and tab-alert glyphs)
// instead of trying to guess every possible prefix in AppleScript.
func (cmux) FocusTab(session string) (string, error) {
	out, err := osa(`tell application id "com.cmuxterm.app"
  set txt to ""
  repeat with terminalItem in terminals
    set txt to txt & (id of terminalItem) & tab & (name of terminalItem) & linefeed
  end repeat
  return txt
end tell`)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		id, title, ok := strings.Cut(line, "\t")
		if !ok || id == "" || !ghostty.TitleMatchesSession(title, session) {
			continue
		}
		result, err := osa(`tell application id "com.cmuxterm.app"
  repeat with terminalItem in terminals
    if id of terminalItem is "` + aplQuote(id) + `" and name of terminalItem is "` + aplQuote(title) + `" then
      focus terminalItem
      activate
      return "ok"
    end if
  end repeat
  return "notfound"
end tell`)
		return result, err
	}
	return "notfound", nil
}

func (cmux) IsViewing(session string) bool {
	out, err := osa(`tell application id "com.cmuxterm.app"
  if it is not frontmost then return ""
  try
    return name of focused terminal of selected tab of front window
  on error
    return ""
  end try
end tell`)
	return err == nil && ghostty.TitleMatchesSession(out, session)
}

func (cmux) TabOrder() []string {
	out, err := osa(`tell application id "com.cmuxterm.app"
  set txt to ""
  repeat with w in windows
    repeat with workspace in tabs of w
      try
        set txt to txt & (name of focused terminal of workspace) & linefeed
      end try
    end repeat
  end repeat
  return txt
end tell`)
	if err != nil {
		return nil
	}
	return ghostty.SessionsFromTitles(out)
}

func (cmux) OpenWindow(command string) (string, error) {
	return osa(`tell application id "com.cmuxterm.app"
  set w to new window
  set terminalItem to focused terminal of selected tab of w
  input text "` + aplQuote(command) + `" to terminalItem
  if not (perform action "send_key:enter" on terminalItem) then error "cmux did not submit command"
  activate
end tell`)
}

func (cmux) SpawnTabs(sessions []string, dryRun bool) (string, error) {
	var script strings.Builder
	script.WriteString(`tell application id "com.cmuxterm.app"
  activate
`)
	for _, session := range sessions {
		command := shellQuote(tmux.Bin) + " attach -t " + shellQuote(session)
		script.WriteString(`  if (count of windows) is 0 then
    set w to new window
    set workspace to selected tab of w
  else
    set w to front window
    set workspace to new tab in w
  end if
  set terminalItem to focused terminal of workspace
  focus terminalItem
  input text "` + aplQuote(command) + `" to terminalItem
  if not (perform action "send_key:enter" on terminalItem) then error "cmux did not submit attach command"
`)
	}
	script.WriteString("end tell")
	plan := script.String()
	if dryRun {
		return plan, nil
	}
	_, err := osa(plan)
	return plan, err
}
