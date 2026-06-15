package menubar

import (
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveGtmux finds the cgo-free `gtmux` CLI the menu-bar app shells out to for
// `agents --json` and `focus`. Lookup order (first hit wins):
//
//  1. $GTMUX_BIN (explicit override)
//  2. a sibling next to this executable — inside Gtmux.app, install-app copies
//     the CLI in beside the menu-bar binary, guaranteeing a version match
//  3. $PATH
//  4. common install locations
//
// Returns "gtmux" as a last resort so exec still has something to try.
func ResolveGtmux() string {
	return resolveBinary("gtmux", "GTMUX_BIN")
}

// ResolveMenubar finds the cgo `gtmux-menubar` binary that install-app wraps in
// Gtmux.app. Same lookup order as ResolveGtmux, keyed on $GTMUX_MENUBAR_BIN.
func ResolveMenubar() string {
	return resolveBinary("gtmux-menubar", "GTMUX_MENUBAR_BIN")
}

func resolveBinary(name, env string) string {
	if p := os.Getenv(env); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		if sib := filepath.Join(filepath.Dir(exe), name); isExecutableFile(sib) {
			return sib
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	home := os.Getenv("HOME")
	for _, p := range []string{
		filepath.Join(home, ".local", "bin", name),
		"/opt/homebrew/bin/" + name,
		"/usr/local/bin/" + name,
	} {
		if isExecutableFile(p) {
			return p
		}
	}
	return name
}

func isExecutableFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}
