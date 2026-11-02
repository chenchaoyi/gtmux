package codexbridge

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// Eligible preserves explicit embedded, remote, profile, provider and headless
// launches. Unknown options remain Codex's responsibility and are never rewritten.
// This is capability admission, not a comparison of version strings.
func Eligible(args []string) bool { return agents.CodexBridgeEligible(args) }

// LocalArgs keeps the terminal's working directory when explicit local transport
// would otherwise inherit the daemon's workspace. User -C/--cd wins unchanged.
func LocalArgs(args []string, cwd string) []string {
	out := append([]string{}, args...)
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "-C" || a == "--cd" || strings.HasPrefix(a, "--cd=") {
			return out
		}
	}
	return append([]string{"--cd", cwd}, out...)
}

// HelpAdmits excludes every native subcommand advertised by the installed CLI,
// including ones introduced after this adapter. Resume/fork are interactive.
// Unknown help layouts stay native rather than inferring a prompt.
func HelpAdmits(args []string, help string) bool {
	commands := map[string]bool{}
	section := false
	for _, line := range strings.Split(help, "\n") {
		if strings.TrimSpace(line) == "Commands:" {
			section = true
			continue
		}
		if !section {
			continue
		}
		if line != "" && line[0] != ' ' {
			break
		}
		fields := strings.Fields(line)
		if len(fields) > 1 && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			commands[fields[0]] = true
		}
		if _, aliases, ok := strings.Cut(line, "[aliases: "); ok {
			aliases, _, _ = strings.Cut(aliases, "]")
			for _, alias := range strings.Split(aliases, ",") {
				commands[strings.TrimSpace(alias)] = true
			}
		}
	}
	if !commands["exec"] || !commands["resume"] || !commands["fork"] {
		return false
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return true
		}
		key, _, inline := strings.Cut(a, "=")
		switch key {
		case "--model", "-m", "--sandbox", "-s", "--ask-for-approval", "-a", "--cd", "-C", "--add-dir", "--image", "-i":
			if !inline {
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "resume" || a == "fork" || !commands[a]
	}
	return true
}
