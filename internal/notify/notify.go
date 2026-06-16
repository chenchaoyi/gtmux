// Package notify posts a desktop notification, preferring terminal-notifier
// (clickable — it can -activate the menu-bar app, which jumps to last-finished)
// and falling back to osascript (not clickable). The notifier is spawned
// DETACHED so it outlives the short-lived hook process that triggered it.
package notify

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Options describes one notification.
type Options struct {
	Title    string // e.g. "Claude Code"
	Subtitle string // e.g. the tmux session name
	Message  string
	Activate string // bundle id to launch on click, e.g. "com.gtmux.menubar"
	Group    string // coalescing key, e.g. "gtmux-<session>"
	IconPath string // -contentImage; omitted when "" (terminal-notifier ignores -appIcon)
}

// Send posts the notification and returns immediately. It never blocks on the
// notifier: the child is detached (its own session, stdio to /dev/null) and not
// waited on, mirroring the bash hook's `nohup … </dev/null &`.
func Send(o Options) {
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		args := []string{"-title", o.Title, "-sound", "default"}
		if o.Activate != "" {
			args = append(args, "-activate", o.Activate)
		}
		if o.IconPath != "" {
			args = append(args, "-contentImage", o.IconPath)
		}
		if o.Subtitle != "" {
			args = append(args, "-subtitle", o.Subtitle)
		}
		args = append(args, "-message", o.Message)
		if o.Group != "" {
			args = append(args, "-group", o.Group)
		}
		spawnDetached(path, args)
		return
	}

	// Fallback: a non-clickable banner via osascript.
	script := "display notification " + osaStr(o.Message) + " with title " + osaStr(o.Title)
	if o.Subtitle != "" {
		script += " subtitle " + osaStr(o.Subtitle)
	}
	spawnDetached("/usr/bin/osascript", []string{"-e", script})
}

// spawnDetached starts bin in its own session with stdio routed to /dev/null and
// does NOT wait, so it survives the parent exiting.
func spawnDetached(bin string, args []string) {
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
		defer devnull.Close()
	}
	_ = cmd.Start() // intentionally not Wait()ed — let it outlive us
}

// osaStr quotes s as an AppleScript string literal.
func osaStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
