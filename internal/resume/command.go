package resume

import "strings"

// resumeArgv is each agent's "continue a specific conversation" command, ported
// from cmux's AgentResumeArgv. The session id is appended as the final argument.
// An agent absent here can't be resumed by id (we won't relaunch it).
var resumeArgv = map[string][]string{
	"claude":       {"claude", "--resume"},
	"codex":        {"codex", "resume"},
	"cursor":       {"cursor-agent", "--resume"},
	"gemini":       {"gemini", "--resume"},
	"kiro":         {"kiro-cli", "chat", "--resume-id"},
	"copilot":      {"copilot", "--resume"},
	"opencode":     {"opencode", "--session"},
	"hermes-agent": {"hermes", "--resume"},
	"grok":         {"grok", "-r"},
}

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
