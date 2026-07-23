package app

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Finding a helper binary when $PATH is not the user's PATH (tool-path-resolution).
//
// A process launched from the GUI — the menu-bar app shelling out, or a LaunchAgent —
// inherits launchd's PATH: `/usr/bin:/bin:/usr/sbin:/sbin`. Homebrew installs to
// `/opt/homebrew/bin` or `/usr/local/bin`, and neither is on it. So `exec.LookPath`
// reports a tool as missing on a machine where the user installed it and can run it from
// their terminal.
//
// That produced a bug with no visible cause: the menu bar could not switch to Anywhere
// because `gtmux tunnel` was told cloudflared "isn't installed" and Homebrew "isn't
// installed to fetch it" — both sitting in /usr/local/bin the whole time. From a terminal
// the identical command worked, which is exactly what makes this class of bug expensive.
//
// internal/tmux already learned this and hardcoded the fallback for tmux alone. This is
// that lesson, generalized, so the next tool doesn't have to learn it again.

// toolSearchDirs are the install locations to try when PATH doesn't have a tool: both
// Homebrew prefixes (Apple Silicon, Intel), the user-local bin gtmux itself installs to,
// and the system dirs (so an explicit search still finds a system tool).
func toolSearchDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"))
	}
	return append(dirs, "/usr/bin", "/bin", "/usr/sbin", "/sbin")
}

// lookTool resolves a helper binary: PATH first (so an override wins), then the known
// install locations. Returns "" when it genuinely isn't installed — the caller can then
// say so and be right.
func lookTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, d := range toolSearchDirs() {
		p := filepath.Join(d, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}
