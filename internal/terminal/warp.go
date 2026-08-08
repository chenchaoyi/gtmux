package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// warp drives the Warp terminal — best-effort, because Warp (verified on
// v0.2026.06, bundle dev.warp.Warp-Stable) ships NO AppleScript scripting
// dictionary (no sdef, no NSAppleScriptEnabled): only the standard suite
// (activate/quit) works, so the tab-title iteration the Ghostty/iTerm2 drivers
// use is impossible. What Warp DOES support today, all verified on a real Warp:
//
//   - `warp://` URIs: `warp://action/new_tab?path=…` / `new_window?path=…`
//     open a tab/window (no command exec), and `warp://session/<uuid>` focuses
//     the exact tab-session — Warp itself advertises this as $WARP_FOCUS_URL
//     inside every tab.
//   - Launch configurations (~/.warp/launch_configurations/*.yaml, opened via
//     `warp://launch/<name>`) CAN exec a command per tab — that's how
//     OpenWindow/SpawnTabs work.
//
// The catch for FocusTab: mapping a tmux session to its Warp session uuid.
// The uuid lives only in the tab's environment ($WARP_TERMINAL_SESSION_UUID),
// which this macOS no longer lets us read from another process (`ps eww` shows
// no env, verified on Darwin 25), and tmux doesn't capture it by default. So
// gtmux's own SpawnTabs attach script records it into the tmux session env
// (set-environment), and FocusTab reads it back — precise focus for tabs gtmux
// opened (restore/hq/new). Tabs the user opened by hand have no recorded uuid;
// there FocusTab degrades to activating the Warp app (the right app, possibly
// the wrong tab — tmux has already selected the right window+pane, so the
// common one-window case still lands correctly). Users can make hand-opened
// tabs precise too:  tmux set -ga update-environment WARP_TERMINAL_SESSION_UUID
// (then any re-attach from a Warp tab records that tab's uuid).
//
// Warp's macOS PROCESS name is "stable" (the binary inside Warp.app), not
// "Warp" — IsViewing must match that.
type warp struct{}

func (warp) Name() string { return "Warp" }

// warpSessionUUID returns the Warp tab-session uuid recorded in a tmux
// session's environment ("" if none): set by our attach script, or by tmux
// itself when the user adds WARP_TERMINAL_SESSION_UUID to update-environment.
func warpSessionUUID(session string) string {
	for _, line := range tmux.Lines("show-environment", "-t", session, "WARP_TERMINAL_SESSION_UUID") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "WARP_TERMINAL_SESSION_UUID="); ok && v != "" {
			return v
		}
	}
	return ""
}

// openURL opens a warp:// URL via /usr/bin/open. Overridable for tests.
var openURL = func(url string) error {
	return exec.Command("/usr/bin/open", url).Run()
}

func (warp) FocusTab(session string) (string, error) {
	// Precise path: the recorded Warp session uuid → warp://session/<uuid>
	// (Warp's own focus URL). Best-effort: a stale uuid is a silent no-op at
	// Warp's end, so always follow with an app-level activate.
	if uuid := warpSessionUUID(session); uuid != "" {
		_ = openURL("warp://session/" + uuid)
	}
	// The standard suite's activate works without a scripting dictionary.
	if _, err := osa(`tell application "Warp" to activate`); err != nil {
		return "", err
	}
	return "ok", nil
}

// IsViewing reports whether Warp is frontmost AND its front window title names
// this session. Warp's process name is "stable"; accept "warp" too in case a
// future build renames it. If Warp doesn't surface the tmux title (it renders
// its own tab UI), the title check fails and we return false — never wrongly
// suppress a notification.
func (warp) IsViewing(session string) bool {
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
	return warpViewingMatch(out, session)
}

// warpViewingMatch decides IsViewing from the System Events "proc\ntitle"
// output: frontmost must be Warp (process name "stable" — the binary inside
// Warp.app — or "warp"), and the window title must name the session (tmux
// set-titles '#S — #W', only visible if Warp surfaces the OSC title).
func warpViewingMatch(out, session string) bool {
	parts := strings.SplitN(out, "\n", 2)
	if len(parts) != 2 {
		return false
	}
	proc := strings.ToLower(strings.TrimSpace(parts[0]))
	if proc != "stable" && proc != "warp" {
		return false
	}
	title := strings.TrimSpace(parts[1])
	return title == session || strings.HasPrefix(title, session+" — ")
}

// TabOrder can't be read: Warp exposes no scriptable tab list. Restore simply
// won't record/replay Warp tab order.
func (warp) TabOrder() []string { return nil }

// launchConfigDir is where Warp reads launch configurations from.
// Overridable for tests.
var launchConfigDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".warp", "launch_configurations")
}

// warpYAMLQuote escapes a string for a double-quoted YAML scalar.
func warpYAMLQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// warpLaunchYAML builds a Warp launch configuration: one window, one tab per
// entry, each tab exec'ing its command.
func warpLaunchYAML(name string, tabs [][2]string) string {
	var b strings.Builder
	b.WriteString("---\nname: " + name + "\nwindows:\n  - tabs:\n")
	for _, t := range tabs {
		b.WriteString("      - title: \"" + warpYAMLQuote(t[0]) + "\"\n")
		b.WriteString("        layout:\n")
		b.WriteString("          commands:\n")
		b.WriteString("            - exec: \"" + warpYAMLQuote(t[1]) + "\"\n")
	}
	return b.String()
}

// warpAttachScript is the per-tab attach command SpawnTabs uses: attach the
// tmux session AND record this tab's Warp session uuid into the tmux session
// env, so a later FocusTab can jump to this exact tab via warp://session/<uuid>.
// Written as a script file because Warp's YAML `exec` doesn't reliably carry
// tmux's `\;` command separator.
const warpAttachScript = `#!/bin/bash
# gtmux: attach a tmux session in this Warp tab, recording the tab's Warp
# session uuid so ` + "`gtmux focus`" + ` can come back to this exact tab.
s="$1"
exec ` + "%s" + ` attach -t "$s" \; set-environment -t "$s" WARP_TERMINAL_SESSION_UUID "${WARP_TERMINAL_SESSION_UUID:-}"
`

// writeLaunchConfig writes a launch configuration (and its attach script when
// needed) and returns the config name to open.
func writeLaunchConfig(name, yaml string) error {
	dir := launchConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0o644)
}

func (warp) OpenWindow(command string) (string, error) {
	yaml := warpLaunchYAML("gtmux-window", [][2]string{{"gtmux", command}})
	if err := writeLaunchConfig("gtmux-window", yaml); err != nil {
		return "", err
	}
	return "", openURL("warp://launch/gtmux-window")
}

func (warp) SpawnTabs(sessions []string, dryRun bool) (string, error) {
	dir := launchConfigDir()
	script := fmt.Sprintf(warpAttachScript, tmux.Bin)
	scriptPath := filepath.Join(dir, "gtmux-attach.sh")
	var tabs [][2]string
	for _, s := range sessions {
		tabs = append(tabs, [2]string{s, "bash " + shellQuote(scriptPath) + " " + shellQuote(s)})
	}
	yaml := warpLaunchYAML("gtmux-restore", tabs)
	if dryRun {
		return yaml, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return yaml, err
	}
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return yaml, err
	}
	if err := writeLaunchConfig("gtmux-restore", yaml); err != nil {
		return yaml, err
	}
	return yaml, openURL("warp://launch/gtmux-restore")
}
