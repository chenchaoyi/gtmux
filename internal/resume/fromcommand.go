package resume

import (
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// Reading a recorded COMMAND LINE back into {agent, conversation id} — the inverse
// of Command().
//
// Why gtmux needs the inverse: a tmux-resurrect save records each pane's
// `pane_full_command` at save time, e.g. `claude --resume <uuid>`. That line is the
// only evidence of what was ACTUALLY running in a pane the moment the machine went
// down — the process itself is long dead by the time `gtmux restore` runs, so no
// amount of process inspection can recover it. Restore uses it two ways: as the
// gate ("was an agent alive here at all?") and, when a pane has no saved resume
// record, as the conversation id to bring back.

// commandKeys maps an executable name to the agent key it identifies (from the
// agent registry, the single source of truth for agent identity).
var commandKeys = agents.CommandKeys()

// interpreters are argv[0]s that merely WRAP the real command — an agent installed
// as a JS/Python entry point shows up as `node /path/to/claude …` in ps output, and
// the agent's name is the next token.
var interpreters = map[string]bool{
	"node": true, "bun": true, "deno": true, "npx": true, "tsx": true,
	"python": true, "python3": true, "ruby": true, "env": true,
}

// shells are interpreters too — an agent shipped as a shebang shell script runs as
// `/bin/sh /usr/local/bin/opencode …` — but they are skipped ONLY when something
// follows. A bare `bash` is a shell, not a wrapper, and callers depend on being able
// to see that: it is the difference between "this pane was idle" and "this pane was
// running an agent".
var shells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
}

// FromCommand identifies which agent a recorded command line launched and, when the
// line carries one, the conversation id it was resuming. Both are "" when the line
// names no agent gtmux knows.
//
//	"claude --resume 369b78a0"        → "claude", "369b78a0"
//	"node /opt/bin/codex resume abc"  → "codex",  "abc"
//	"claude"                          → "claude", ""      (fresh session, no id)
//	"bash" / "vim notes.md"           → "",       ""
func FromCommand(cmdline string) (agent, sessionID string) {
	toks := strings.Fields(cmdline)
	i := 0
	for i < len(toks) {
		t := toks[i]
		// `VAR=value cmd …` — an env prefix, not the command.
		if !strings.HasPrefix(t, "-") && strings.Contains(t, "=") {
			i++
			continue
		}
		base := strings.TrimPrefix(filepath.Base(t), "-") // login shells report as "-bash"
		if interpreters[base] {
			i++
			continue
		}
		if shells[base] && i+1 < len(toks) {
			i++
			continue
		}
		break
	}
	if i >= len(toks) {
		return "", ""
	}
	key, ok := commandKeys[filepath.Base(toks[i])]
	if !ok {
		return "", ""
	}
	return key, sessionIDFromArgs(key, toks[i+1:])
}

// sessionIDFromArgs pulls the conversation id out of an agent's arguments, using the
// SAME argv table Command() builds with (agents.ResumeArgv) — so an agent whose
// resume flag differs (`codex resume <id>`, `opencode --session <id>`) is read back
// correctly without a second table to keep in sync.
func sessionIDFromArgs(key string, args []string) string {
	argv, ok := resumeArgv[key]
	if !ok || len(argv) < 2 {
		return ""
	}
	flag := argv[len(argv)-1] // the token the id follows: --resume / resume / --session / -r
	for i, a := range args {
		if a == flag {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return unquote(args[i+1])
			}
			return ""
		}
		if v, found := strings.CutPrefix(a, flag+"="); found && v != "" {
			return unquote(v) // --resume=<id>
		}
	}
	return ""
}

// unquote strips one layer of matching surrounding quotes. A `ps` line shows the
// exec'd argv (already unquoted), but the same parser reads command lines that never
// went through a shell — Command()'s own output among them — so the round trip
// shouldn't depend on which side of the shell the string came from.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
