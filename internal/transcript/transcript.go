// Package transcript turns an agent's on-disk conversation log into a compact
// list of Turns — {prompt, final response, collapsed middle steps} — for the
// mobile/web "对话 (chat)" view. Each agent writes its log in its OWN format, so
// there is one parser per agent (claude.go, codex.go) over a SHARED output model.
//
// The pane→session mapping lives elsewhere (resume.Record gives agent+sessionId);
// here we only take that record and read the log. Files are looked up by GLOB on
// the session id, so we never have to reconstruct an agent's cwd-encoding scheme.
package transcript

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
)

// Step is one collapsed middle action in a turn (a tool call). The chat view
// shows turns as prompt → (N steps, collapsed) → final response.
type Step struct {
	Kind   string `json:"kind"`             // "tool" for now
	Title  string `json:"title"`            // tool name (Edit, Bash, exec_command…)
	Detail string `json:"detail,omitempty"` // a short arg summary (path / command head)
}

// Turn is one user instruction and what the agent ultimately answered, with the
// intermediate tool calls folded into Steps.
type Turn struct {
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
	Steps    []Step `json:"steps,omitempty"`
}

// maxTailBytes bounds how much of a (potentially huge) log we read: only the tail
// matters for "recent history", and a partial first turn is simply dropped. 8 MiB
// covers a lot of turns (so the chat view can show deep history); a single
// hyperactive in-progress turn can still exceed it, in which case it surfaces as
// one prompt-less "current activity" card.
const maxTailBytes = 8 << 20 // 8 MiB

// Load returns up to maxTurns most-recent turns (oldest-first) for an agent
// session, dispatching to the per-agent parser. Unknown agent or missing log →
// (nil, nil): the caller renders a friendly "no history yet" state, not an error.
func Load(agent, sessionID string, maxTurns int) ([]Turn, error) {
	if sessionID == "" {
		return nil, nil
	}
	switch normalizeAgent(agent) {
	case "claude":
		return loadClaude(sessionID, maxTurns)
	case "codex":
		return loadCodex(sessionID, maxTurns)
	}
	return nil, nil
}

// normalizeAgent maps a display name or key ("Claude Code", "claude", "Codex")
// to a parser key.
func normalizeAgent(agent string) string {
	a := strings.ToLower(agent)
	switch {
	case strings.Contains(a, "claude"):
		return "claude"
	case strings.Contains(a, "codex"):
		return "codex"
	}
	return a
}

// tailLines reads the last maxTailBytes of a file and returns its complete lines
// (the first, possibly-truncated line is dropped unless we read from the start).
func tailLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	start := int64(0)
	truncated := false
	if size > maxTailBytes {
		start = size - maxTailBytes
		truncated = true
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if truncated {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			data = data[i+1:] // drop the partial first line
		}
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // tolerate very long lines
	for sc.Scan() {
		if l := sc.Text(); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines, sc.Err()
}

// lastN trims turns to the most recent n (oldest-first order preserved).
func lastN(turns []Turn, n int) []Turn {
	if n > 0 && len(turns) > n {
		return turns[len(turns)-n:]
	}
	return turns
}

// clip shortens s to max runes with an ellipsis, collapsing whitespace — for a
// step's one-line detail.
func clip(s string, max int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
