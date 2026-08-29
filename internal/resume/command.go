package resume

import (
	"strings"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

// resumeArgv is each agent's "continue a specific conversation" command. The
// session id is appended as the final argument. An agent absent here can't be
// resumed by id (we won't relaunch it). Sourced from the agent registry, the
// single source of truth (agents.ResumeArgv).
var resumeArgv = agents.ResumeArgv()

// Resumable reports whether an agent can be relaunched by session id.
func Resumable(agent string) bool {
	_, ok := resumeArgv[agent]
	return ok
}

// Command builds the shell command that relaunches a record's conversation. It
// prepends a cwd guard (cmux's pattern) so the agent resumes IN its original
// working directory — critical: agents file their transcript under the launch
// dir, so `--resume` only finds the conversation when run from there. The guard
// still succeeds if the dir is gone. Returns ok=false when the record isn't
// resumable (unknown agent or no session id).
func Command(r Record) (string, bool) {
	argv, ok := resumeArgv[r.Agent]
	if !ok || r.SessionID == "" {
		return "", false
	}
	// Resume through the launcher the session was actually started with, when one was
	// recorded. Only the executable is substituted — the agent's own resume flags still
	// come from the registry, because a wrapper forwards its arguments to the agent, it
	// does not redefine them.
	if r.Launcher != "" {
		argv = append([]string{r.Launcher}, argv[1:]...)
	}
	cmd := strings.Join(argv, " ") + " " + shellSingleQuote(r.SessionID)
	if r.Cwd != "" {
		q := shellSingleQuote(r.Cwd)
		cmd = "{ cd -- " + q + " 2>/dev/null || [ ! -d " + q + " ]; } && " + cmd
	}
	return cmd, true
}

// shellSingleQuote wraps s in single quotes, escaping any embedded single quote
// the POSIX way ('\” ends the quote, adds an escaped quote, reopens).
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
