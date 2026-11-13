package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// OpenCode persists its native state in version-specific databases. The installed
// bridge records a bounded, portable transcript for both V1 and V2 without
// querying or migrating a live database. Legacy text records remain readable.

// opencodeDir is where the gtmux-owned opencode transcripts live — a sibling of the
// other state under ~/.local/share/gtmux (kept in sync with internal/state.Dir(),
// replicated here so this package stays a pure leaf).
// OpencodeDir is where gtmux keeps the transcripts it writes for opencode (a logless
// agent), for packages that scan them.
func OpencodeDir() string { return opencodeDir() }

func opencodeDir() string {
	return filepath.Join(state.Dir(), "octrans")
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
	Step      *Step  `json:"step,omitempty"`
	CallID    string `json:"call_id,omitempty"`
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
	appendOpencodeLine(sessionID, opencodeLine{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Role: role, Text: text})
}

func appendOpencodeLine(sessionID string, rec opencodeLine) {
	if sessionID == "" {
		return
	}
	if err := state.MkdirAll(opencodeDir(), 0o700); err != nil {
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	f, err := state.OpenFile(opencodeSessionFile(sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
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
	case "tool":
		if e.Step != nil {
			e.Step.fullInput, e.Step.fullWorkdir = e.Step.Input, e.Step.Workdir
			e.Step.Input, e.Step.DetailsTruncated = boundedToolText(e.Step.Input, maxToolInputBytes)
			st.addRecordedStep("opencode", *e.Step, e.CallID)
		}
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

// AppendOpencodeEvent admits selected inputs and text, never output, reasoning,
// native metadata or arbitrary configuration. Full edit inputs use the same
// scoped source-position reader as every other transcript adapter.
func AppendOpencodeEvent(sessionID string, raw json.RawMessage) {
	var event struct {
		Kind, ID, Name, Text string
		Input                json.RawMessage
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	switch event.Kind {
	case "text":
		AppendOpencode(sessionID, "assistant", event.Text)
	case "tool":
		if event.Name == "" {
			return
		}
		step := providerToolStep("opencode", event.Name, event.Name, event.Input, kimiToolDetail(string(event.Input)))
		step.Input, step.Workdir = step.fullInput, step.fullWorkdir
		step.DetailsTruncated = false
		appendOpencodeLine(sessionID, opencodeLine{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Role: "tool", Step: &step, CallID: event.ID})
	}
}
