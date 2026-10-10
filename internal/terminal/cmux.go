package terminal

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// cmux is a Ghostty-based terminal, but its workspace and terminal controls
// belong to cmux. Never send cmux sessions through Ghostty's driver.
//
// Focus prefers the cmux CLI tree (TTY → surface id): AppleScript's terminal
// `name` is often the literal "Terminal", so title matching alone cannot jump
// (issue #1542). AppleScript still performs the focus-by-id step, and remains
// the fallback when the CLI tree is unavailable. Workspace creation also uses
// the AppleScript dictionary — the CLI socket can reject callers outside cmux.
type cmux struct{}

func (cmux) Name() string { return "cmux" }

func (cmux) FocusTab(session string) (string, error) {
	if id := cmuxSurfaceIDForSession(session); id != "" {
		return focusCmuxTerminal(id)
	}
	// Fallback: AppleScript names (works only when cmux exposes real titles).
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
	var onlyID string
	n := 0
	for _, line := range strings.Split(out, "\n") {
		id, title, ok := strings.Cut(line, "\t")
		if !ok || id == "" {
			continue
		}
		n++
		onlyID = id
		if ghostty.TitleMatchesSession(title, session) {
			return focusCmuxTerminal(id)
		}
	}
	// Last resort: one panel and this session is attached somewhere in cmux —
	// that panel is the only place the session can be showing.
	if n == 1 && onlyID != "" && len(clientTTYsFor(session)) > 0 {
		return focusCmuxTerminal(onlyID)
	}
	return "notfound", nil
}

func focusCmuxTerminal(id string) (string, error) {
	return osa(`tell application id "com.cmuxterm.app"
  repeat with terminalItem in terminals
    if id of terminalItem is "` + aplQuote(id) + `" then
      focus terminalItem
      activate
      return "ok"
    end if
  end repeat
  return "notfound"
end tell`)
}

func (cmux) IsViewing(session string) bool {
	surfaces, activeID, err := listCmuxSurfaces()
	if err == nil {
		want := map[string]bool{}
		for _, t := range clientTTYsFor(session) {
			want[t] = true
		}
		for _, s := range surfaces {
			if activeID != "" && s.ID != activeID {
				continue
			}
			if activeID == "" && !s.Focused {
				continue
			}
			if s.TTY != "" && want[s.TTY] {
				return true
			}
			if ghostty.TitleMatchesSession(s.Title, session) {
				return true
			}
			if activeID != "" {
				return false
			}
		}
		if activeID != "" {
			return false
		}
	}
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
	surfaces, _, err := listCmuxSurfaces()
	if err == nil && len(surfaces) > 0 {
		ttySessions := allClientTTYSessions()
		var out []string
		seen := map[string]bool{}
		for _, s := range surfaces {
			if sess := ttySessions[s.TTY]; sess != "" {
				if !seen[sess] {
					seen[sess] = true
					out = append(out, sess)
				}
				continue
			}
			for _, name := range ghostty.SessionsFromTitles(s.Title + "\n") {
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
			}
		}
		return out
	}
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
