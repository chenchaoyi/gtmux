// Package toolpath finds a helper binary when $PATH is not the user's PATH.
package toolpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
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
//
// It lives in its OWN package because "the next tool" turned out to be `gh`, in
// internal/dispatch — a LEAF, which cannot import internal/app. So the generalized
// lesson was unreachable from where it was next needed, and `gtmux reap` re-learned it
// the expensive way: its merged-check shelled `gh` through exec.Command, got "not found"
// under launchd's PATH, and reported a squash-merged branch as UNMERGED (see
// docs/TROUBLESHOOTING.md). A shared lesson has to sit where every layer can reach it.

// SearchDirs are the install locations to try when PATH doesn't have a tool: both
// Homebrew prefixes (Apple Silicon, Intel), the user-local bin gtmux itself installs to,
// and the system dirs (so an explicit search still finds a system tool).
func SearchDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin"}
	if home := state.Home(); home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"))
	}
	return append(append(dirs, nodeVersionDirs()...), "/usr/bin", "/bin", "/usr/sbin", "/sbin")
}

// Look resolves a helper binary: PATH first (so an override wins), then the known
// install locations. Returns "" when it genuinely isn't installed — the caller can then
// say so and be right.
func Look(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, d := range SearchDirs() {
		p := filepath.Join(d, name)
		if IsExecutable(p) {
			return p
		}
	}
	return ""
}

// GUI processes do not load nvm's shell function. Discover installed versions
// without executing shell startup files; prefer the configured default alias.
func nodeVersionDirs() []string {
	root := os.Getenv("NVM_DIR")
	if root == "" {
		root = filepath.Join(state.Home(), ".nvm")
	}
	entries, err := os.ReadDir(filepath.Join(root, "versions", "node"))
	if err != nil {
		return nil
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && nodeVersion(e.Name()) != [3]int{} {
			versions = append(versions, e.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		a, b := nodeVersion(versions[i]), nodeVersion(versions[j])
		for k := range a {
			if a[k] != b[k] {
				return a[k] > b[k]
			}
		}
		return versions[i] < versions[j]
	})
	alias, _ := os.ReadFile(filepath.Join(root, "alias", "default"))
	prefix := strings.TrimPrefix(strings.TrimSpace(string(alias)), "v")
	var preferred, rest []string
	for _, v := range versions {
		dir := filepath.Join(root, "versions", "node", v, "bin")
		n := strings.TrimPrefix(v, "v")
		if prefix != "" && (n == prefix || strings.HasPrefix(n, prefix+".")) {
			preferred = append(preferred, dir)
		} else {
			rest = append(rest, dir)
		}
	}
	return append(preferred, rest...)
}

func nodeVersion(name string) [3]int {
	var version [3]int
	if !strings.HasPrefix(name, "v") {
		return version
	}
	parts := strings.Split(strings.TrimPrefix(name, "v"), ".")
	if len(parts) != 3 {
		return version
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}
		}
		version[i] = n
	}
	return version
}

// IsExecutable checks a resolved file without invoking it.
func IsExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Mode()&0111 != 0
}
