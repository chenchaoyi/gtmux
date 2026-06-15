package main

// version is the gtmux release version. Overridable at build time via
// -ldflags "-X main.version=<v>" (e.g. from GoReleaser when published).
var version = "0.0.1"

// tagline — what gtmux is, in one line (positioning, not implementation).
func tagline() string {
	return tr(
		"command center for your tmux sessions and coding agents",
		"tmux 会话与 coding agent 的指挥台")
}
