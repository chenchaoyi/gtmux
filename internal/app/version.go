package app

import "github.com/chenchaoyi/gtmux/internal/i18n"

// Version is the gtmux release version. Overridable at build time via
// -ldflags "-X github.com/chenchaoyi/gtmux/internal/app.Version=<v>"
// (e.g. from GoReleaser when published).
var Version = "0.0.1"

// tagline — what gtmux is, in one line (positioning, not implementation).
func tagline() string {
	return i18n.Tr(
		"command center for your tmux sessions and coding agents",
		"tmux 会话与 coding agent 的指挥台")
}
