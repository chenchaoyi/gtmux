package resume

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// Resolving a record against the agent's OWN transcript store, because the cwd we
// recorded is not necessarily where the conversation can be resumed from.
//
// The bug this fixes: a coding agent files its transcript under the directory the
// session STARTED in, but the agent can `cd` elsewhere mid-session — and the cwd we
// captured from the pane/hook follows that move. Restore then ran
// `cd <moved-to-dir> && claude --resume <id>` and the agent answered "No conversation
// found with session ID", because the conversation lives under the ORIGINAL dir.
//
// Claude Code stores each conversation at
// `~/.claude/projects/<path-with-slashes-as-dashes>/<session-id>.jsonl`, and the
// transcript records the session's authoritative `cwd`. We read that rather than
// decoding the directory name, because the encoding is lossy — a real directory named
// `a-b` and the path `a/b` both encode to `-a-b`, so decoding cannot be trusted.

// transcriptScanLines caps how far into a transcript we look for the cwd field. The
// first record is a small header without one, so we scan a little, not the whole file.
const transcriptScanLines = 200

// claudeProjectsDir is where Claude Code keeps per-project transcripts. Split out so
// tests can point HOME at a fixture.
func claudeProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// transcriptCwd finds a claude conversation by session id and returns the cwd it is
// filed under — the ONLY directory `--resume` will find it from. found=false means the
// conversation is not on disk at all (deleted/expired), so resuming it can only fail.
func transcriptCwd(sessionID string) (cwd string, found bool) {
	dir := claudeProjectsDir()
	if dir == "" || sessionID == "" {
		return "", false
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*", sessionID+".jsonl"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	// The transcript exists. Read its recorded cwd; if the file somehow carries none,
	// still report found — the conversation is real, we just can't improve on the
	// caller's cwd.
	return cwdFromTranscript(matches[0]), true
}

// cwdFromTranscript reads the first `cwd` recorded in a transcript ("" if none).
func cwdFromTranscript(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // transcript lines can be large
	for i := 0; i < transcriptScanLines && sc.Scan(); i++ {
		var rec struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Cwd != "" {
			return rec.Cwd
		}
	}
	return ""
}

// Resolve corrects a record so it can actually be resumed, and reports whether the
// conversation still exists.
//
// For an agent whose store we can inspect (claude today) it rewrites Cwd to the
// directory the conversation is filed under — fixing the "agent cd'd mid-session"
// failure — and returns alive=false when the conversation is gone, so the caller can
// skip a doomed `--resume` instead of leaving an error on the user's screen.
//
// For any other agent it is a no-op returning alive=true: we can't verify their layout,
// so behavior is unchanged.
func Resolve(r Record) (Record, bool) {
	if r.Agent != "claude" || r.SessionID == "" {
		return r, true
	}
	cwd, found := transcriptCwd(r.SessionID)
	if !found {
		return r, false // conversation is gone — resuming it can only error
	}
	if cwd != "" {
		r.Cwd = cwd // resume from where the conversation actually lives
	}
	return r, true
}
