//go:build darwin

package main

// version is the gtmux-menubar release version, injected at build time via
// -ldflags "-X main.version=<v>". "dev" for local / un-stamped builds.
// Darwin-only: only the real menu-bar app (main_darwin.go) uses it; the Linux
// stub doesn't, and an unused package var would fail staticcheck there.
var version = "dev"
