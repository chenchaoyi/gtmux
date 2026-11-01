package ghostty

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const tabOrderTimeout = 4 * time.Second

// Bulk property reads avoid one AppleEvent per tab. Keep a bounded fallback for
// older Ghostty scripting dictionaries; both attempts share the same deadline.
const tabOrderScript = `if application "Ghostty" is not running then return ""
with timeout of 3 seconds
  tell application "Ghostty" to set titleGroups to name of every tab of every window
end timeout
set txt to ""
repeat with titleGroup in titleGroups
  if class of (contents of titleGroup) is list then
    repeat with tabTitle in titleGroup
      set txt to txt & (tabTitle as text) & linefeed
    end repeat
  else
    set txt to txt & (titleGroup as text) & linefeed
  end if
end repeat
return txt`

const legacyTabOrderScript = `if application "Ghostty" is not running then return ""
with timeout of 3 seconds
  tell application "Ghostty"
    set txt to ""
    repeat with w in windows
      repeat with t in tabs of w
        set txt to txt & (name of t) & linefeed
      end repeat
    end repeat
    return txt
  end tell
end timeout`

type tabOrderReadError struct{ reason string }

func (e tabOrderReadError) Error() string          { return "tab order unavailable: " + e.reason }
func (e tabOrderReadError) TabOrderReason() string { return e.reason }

func tabOrderError(err error, stderr string) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(stderr, "(-1712)"):
		return tabOrderReadError{"timeout"}
	case strings.Contains(stderr, "(-1743)"):
		return tabOrderReadError{"automation-denied"}
	default:
		return tabOrderReadError{"script"}
	}
}

func runTabOrderScript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", tabOrderError(ctx.Err(), "")
		}
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			return "", tabOrderError(err, string(exited.Stderr))
		}
		return "", tabOrderError(err, "")
	}
	return scriptOutput(out), nil
}

func readTabOrder(ctx context.Context, run func(context.Context, string) (string, error)) ([]string, error) {
	out, err := run(ctx, tabOrderScript)
	var failed tabOrderReadError
	if err != nil && errors.As(err, &failed) && failed.reason == "script" && ctx.Err() == nil {
		out, err = run(ctx, legacyTabOrderScript)
	}
	if err != nil {
		return nil, err
	}
	return SessionsFromTitles(out), nil
}

func (Driver) ReadTabOrder() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tabOrderTimeout)
	defer cancel()
	return readTabOrder(ctx, runTabOrderScript)
}
