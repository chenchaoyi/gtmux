package terminal

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// resolveName picks the host terminal's driver name. Order:
//  1. GTMUX_TERMINAL — explicit override.
//  2. this process's terminal env — set when gtmux runs INSIDE a terminal
//     (e.g. `gtmux restore` / `new` typed in iTerm2 → TERM_PROGRAM=iTerm.app).
//  3. the tmux client's process ancestry — for `focus` invoked by the menu-bar
//     app (no TERM_PROGRAM), the attached tmux client's terminal app.
//  4. "ghostty" — default / fallback.
func resolveName() string {
	if v := os.Getenv("GTMUX_TERMINAL"); v != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	if n := fromTermEnv(); n != "" {
		return n
	}
	if n := fromTmuxClient(); n != "" {
		return n
	}
	return "ghostty"
}

// fromTermEnv maps this process's terminal environment to a driver name.
func fromTermEnv() string {
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty":
		return "ghostty"
	case "iTerm.app":
		return "iterm2"
	case "Apple_Terminal":
		return "appleterminal"
	case "WezTerm":
		return "wezterm"
	case "WarpTerminal":
		return "warp"
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" {
		return "kitty"
	}
	return ""
}

// fromTmuxClient walks each attached tmux client's process ancestry up to the
// terminal app hosting it. "" if no client or no recognized terminal.
func fromTmuxClient() string {
	for _, line := range tmux.Lines("list-clients", "-F", "#{client_pid}") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		if n := terminalFromAncestry(pid); n != "" {
			return n
		}
	}
	return ""
}

// terminalFromAncestry walks the parent chain from pid and returns the first
// recognized terminal app's driver name.
func terminalFromAncestry(pid int) string {
	for depth := 0; depth < 12 && pid > 1; depth++ {
		out, err := exec.Command("ps", "-o", "ppid=,command=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return ""
		}
		fs := strings.Fields(string(out))
		if len(fs) < 2 {
			return ""
		}
		ppid, _ := strconv.Atoi(fs[0])
		if n := terminalFromCommand(strings.Join(fs[1:], " ")); n != "" {
			return n
		}
		pid = ppid
	}
	return ""
}

// terminalFromCommand maps a process command line to a terminal driver name.
func terminalFromCommand(cmd string) string {
	lc := strings.ToLower(cmd)
	switch {
	case strings.Contains(lc, "ghostty"):
		return "ghostty"
	case strings.Contains(lc, "iterm.app") || strings.Contains(lc, "/iterm"):
		return "iterm2"
	case strings.Contains(cmd, "Terminal.app"):
		return "appleterminal"
	case strings.Contains(lc, "kitty"):
		return "kitty"
	case strings.Contains(lc, "wezterm"):
		return "wezterm"
	case strings.Contains(lc, "warp.app") || strings.Contains(lc, "/warp"):
		return "warp"
	}
	return ""
}
