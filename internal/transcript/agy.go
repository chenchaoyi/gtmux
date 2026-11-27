package transcript

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Antigravity CLI (`agy`) writes one conversation per UUID directory under
// $AGY_HOME/brain/<conversation-id>/.system_generated/logs/transcript.jsonl (default
// ~/.gemini/antigravity-cli). Each line is a flat step object:
//
//   - source:"USER_EXPLICIT", type:"USER_INPUT" — user turn prompt (often wrapped in
//     <USER_REQUEST>...</USER_REQUEST> alongside <ADDITIONAL_METADATA>).
//   - source:"MODEL", type:"PLANNER_RESPONSE" — model step carrying created_at, model,
//     token counters, optional tool_calls, and final content (status:"DONE"|"ERROR").
//   - source:"MODEL", type:"GENERIC" — tool execution output (skipped by turn reader).
//   - source:"SYSTEM", type:"SYSTEM_MESSAGE"|"ERROR_MESSAGE" — harness reminders and errors.

// AgyHome is Antigravity CLI's data root, honoring AGY_HOME for relocation and testing.
func AgyHome() string { return agyHome() }

func agyHome() string {
	if h := os.Getenv("AGY_HOME"); h != "" {
		return h
	}
	return filepath.Join(state.Home(), ".gemini", "antigravity-cli")
}

func safeAgySessionID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`) && !strings.Contains(id, "..")
}

func agyLogPath(sessionID string) string {
	if !safeAgySessionID(sessionID) {
		return ""
	}
	base := filepath.Join(agyHome(), "brain", sessionID, ".system_generated", "logs")
	for _, name := range []string{"transcript.jsonl", "transcript_full.jsonl"} {
		if p := filepath.Join(base, name); fileExists(p) {
			return p
		}
	}
	return ""
}

type agyToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type agyLine struct {
	StepIndex int           `json:"step_index"`
	Source    string        `json:"source"`
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	Error     string        `json:"error"`
	CreatedAt string        `json:"created_at"`
	Model     string        `json:"model"`
	Content   string        `json:"content"`
	ToolCalls []agyToolCall `json:"tool_calls"`
}

func agyStep(line string, st *parseState) {
	var r agyLine
	if json.Unmarshal([]byte(line), &r) != nil {
		return
	}
	switch r.Type {
	case "USER_INPUT":
		if r.Source != "" && r.Source != "USER_EXPLICIT" {
			return
		}
		if text := CleanAgyUserContent(r.Content); text != "" {
			st.open(text, r.CreatedAt)
		}
	case "PLANNER_RESPONSE":
		if r.Source != "" && r.Source != "MODEL" {
			return
		}
		if strings.EqualFold(r.Status, "ERROR") {
			return
		}
		for _, tc := range r.ToolCalls {
			if tc.Name == "" {
				continue
			}
			st.addRecordedStep("agy", agyToolStep(tc), tc.ID)
		}
		if text := strings.TrimSpace(r.Content); text != "" {
			st.addText(text)
		}
	}
}

// CleanAgyUserContent extracts the human prompt from Antigravity's USER_INPUT wrapper,
// stripping harness tags and returning "" for compaction-only summaries.
func CleanAgyUserContent(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	const openTag = "<USER_REQUEST>"
	const closeTag = "</USER_REQUEST>"
	if start := strings.Index(s, openTag); start >= 0 {
		rest := s[start+len(openTag):]
		if end := strings.Index(rest, closeTag); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
		return strings.TrimSpace(rest)
	}
	if strings.HasPrefix(s, "<CONTEXT_SUMMARY>") {
		return ""
	}
	for _, marker := range []string{"<ADDITIONAL_METADATA>", "<USER_SETTINGS_CHANGE>"} {
		if idx := strings.Index(s, marker); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return s
}

func agyToolStep(tc agyToolCall) Step {
	normRaw, detail := normalizeAgyToolArgs(tc.Name, tc.Args)
	return providerToolStep("agy", tc.Name, agyToolTitle(tc.Name), normRaw, detail)
}

func agyToolTitle(name string) string {
	switch name {
	case "run_command":
		return "Shell"
	case "read_file":
		return "Read"
	case "edit_file":
		return "Edit"
	case "write_file":
		return "Write"
	case "search_web":
		return "WebSearch"
	case "invoke_subagent":
		return "Subagent"
	default:
		return name
	}
}

// normalizeAgyToolArgs maps agy's PascalCase tool arguments (CommandLine, Path, Target,
// Replacement, Content, Prompt) onto the canonical field names toolStep inspects while
// producing a compact one-line summary for collapsed steps.
func normalizeAgyToolArgs(name string, raw json.RawMessage) (json.RawMessage, string) {
	var args map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &args) != nil {
		return raw, ""
	}
	str := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := args[k].(string); ok {
				s := strings.TrimSpace(v)
				// agy's compact transcript.jsonl occasionally quotes scalar strings twice.
				if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
					var unq string
					if json.Unmarshal([]byte(s), &unq) == nil {
						s = strings.TrimSpace(unq)
					}
				}
				if s != "" {
					return s
				}
			}
		}
		return ""
	}

	norm := map[string]any{}
	for k, v := range args {
		norm[k] = v
	}
	if cmd := str("CommandLine", "command"); cmd != "" {
		norm["command"] = cmd
	}
	if p := str("Path", "path"); p != "" {
		norm["path"] = p
	}
	if t := str("Target", "old_string"); t != "" {
		norm["old_string"] = t
	}
	if rep := str("Replacement", "new_string"); rep != "" {
		norm["new_string"] = rep
	}
	if c := str("Content", "content"); c != "" {
		norm["content"] = c
	}
	if pr := str("Prompt", "prompt"); pr != "" {
		norm["prompt"] = pr
	}
	if q := str("query", "Query"); q != "" {
		norm["query"] = q
	}
	if d := str("toolSummary", "Description", "Role"); d != "" {
		norm["description"] = d
	}

	detail := ""
	switch name {
	case "run_command":
		detail = clip(str("CommandLine", "toolSummary", "command"), 80)
	case "read_file", "edit_file", "write_file":
		detail = clip(str("Path", "path"), 80)
	case "search_web":
		detail = clip(str("query", "Query"), 80)
	case "invoke_subagent":
		detail = clip(str("Role", "Prompt", "TypeName"), 80)
	default:
		detail = clip(str("toolSummary", "Path", "CommandLine", "query", "Description", "Action"), 80)
	}
	encoded, err := json.Marshal(norm)
	if err != nil {
		return raw, detail
	}
	return encoded, detail
}

func agyMessageTimeIn(buf string, last bool) int64 {
	var out int64
	for _, ln := range strings.Split(buf, "\n") {
		if !strings.Contains(ln, `"USER_INPUT"`) && !strings.Contains(ln, `"PLANNER_RESPONSE"`) {
			continue
		}
		var r struct {
			Type      string `json:"type"`
			CreatedAt string `json:"created_at"`
		}
		if json.Unmarshal([]byte(ln), &r) != nil || (r.Type != "USER_INPUT" && r.Type != "PLANNER_RESPONSE") {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
		if err != nil {
			continue
		}
		out = t.Unix()
		if !last {
			return out
		}
	}
	return out
}

func agyLastMessageError(path string) (bool, string) {
	if path == "" {
		return false, ""
	}
	f, err := os.Open(path)
	if err != nil {
		return false, ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, ""
	}
	const tail = 64 << 10
	start := int64(0)
	if fi.Size() > tail {
		start = fi.Size() - tail
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return false, ""
	}
	lines := strings.Split(string(buf), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			continue
		}
		var r agyLine
		if json.Unmarshal([]byte(s), &r) != nil {
			continue
		}
		switch r.Type {
		case "ERROR_MESSAGE":
			return true, clip(r.Error, 80)
		case "PLANNER_RESPONSE":
			if strings.EqualFold(r.Status, "ERROR") {
				return true, clip(r.Error, 80)
			}
			return false, ""
		case "USER_INPUT":
			return false, ""
		}
	}
	return false, ""
}
