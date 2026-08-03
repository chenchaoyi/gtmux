package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// opencode has NO readable on-disk conversation log — 1.18.x persists only a
// session_diff, not messages — so its transcript is gtmux-OWNED. The opencode
// plugin pipes each user prompt and the final assistant text through `gtmux hook`,
// and the hook appends them here as {timestamp, role, text} JSONL, one file per
// opencode session id. This is the ONE agent whose transcript gtmux writes itself;
// every other parser reads a log the agent already keeps. The `"timestamp"` field
// is RFC3339 so LastMessageTime/FirstMessageTime work over it unchanged.

// opencodeDir is where the gtmux-owned opencode transcripts live — a sibling of the
// other state under ~/.local/share/gtmux (kept in sync with internal/state.Dir(),
// replicated here so this package stays a pure leaf).
func opencodeDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "octrans")
}

// opencodeSessionFile flattens a session id into a filename. Session ids are opaque
// `ses_…` tokens; guard against path separators regardless.
func opencodeSessionFile(sessionID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(sessionID)
	return filepath.Join(opencodeDir(), safe+".jsonl")
}

func opencodeLogPath(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	p := opencodeSessionFile(sessionID)
	if _, err := os.Stat(p); err != nil {
		return "" // no transcript yet → caller renders "no history"
	}
	return p
}

// opencodeLine is one appended message record.
type opencodeLine struct {
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"` // "user" | "assistant"
	Text      string `json:"text"`
}

// AppendOpencode records one opencode message — called by `gtmux hook` from the
// plugin-piped payload (see internal/hook). role is "user" or "assistant"; empty
// text or session id is a no-op, as is any other role. Best-effort: a write failure
// is swallowed, since a hook must never fail the agent's turn.
func AppendOpencode(sessionID, role, text string) {
	if sessionID == "" || strings.TrimSpace(text) == "" {
		return
	}
	if role != "user" && role != "assistant" {
		return
	}
	if err := os.MkdirAll(opencodeDir(), 0o755); err != nil {
		return
	}
	rec := opencodeLine{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Role: role, Text: text}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := os.OpenFile(opencodeSessionFile(sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// opencodeStep folds one gtmux-owned opencode transcript line into the parse state:
// a user line opens a turn, an assistant line adds a reply bubble.
func opencodeStep(line string, st *parseState) {
	var e opencodeLine
	if json.Unmarshal([]byte(line), &e) != nil {
		return
	}
	switch e.Role {
	case "user":
		if t := strings.TrimSpace(e.Text); t != "" {
			st.open(t, e.Timestamp)
		}
	case "assistant":
		if t := strings.TrimSpace(e.Text); t != "" {
			st.addText(t)
		}
	}
}
