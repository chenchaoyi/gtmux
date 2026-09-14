package transcript

import "path/filepath"

// LogGlobs names every transcript log on this machine, per agent key — the patterns
// behind LogPath, for a caller that has to walk ALL logs rather than find one session's
// (the daily token ledger, usage-daily-totals). Kept beside the per-agent path helpers
// so a new agent's log location is declared once.
func LogGlobs() map[string][]string {
	return map[string][]string{
		"claude": {filepath.Join(claudeProjectsDir(), "*", "*.jsonl")},
		"codex": {
			filepath.Join(codexHome(), "sessions", "*", "*", "*", "rollout-*.jsonl"),
			filepath.Join(codexHome(), "archived_sessions", "rollout-*.jsonl"),
		},
		"kimi":     {filepath.Join(kimiHome(), "sessions", "*", "*", "agents", "main", "wire.jsonl")},
		"opencode": {filepath.Join(opencodeDir(), "*.jsonl")},
	}
}
