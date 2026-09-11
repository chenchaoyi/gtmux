package mine

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// Codex rollouts live at $CODEX_HOME/sessions/YYYY/MM/DD/rollout-<ts>-<id>.jsonl; each
// line is {timestamp, ordinal, type, payload}. What the miner reads, verified against
// the rollouts on the machine this was written on (84 files, several CLI versions):
//
//   - session_meta        → session id + cwd (first line)
//   - response_item/message, role user|assistant, content[].text — the conversation.
//     internal/transcript reads the `event_msg` user_message/agent_message stream
//     instead, but recent Codex versions stop writing those (the measured rollouts
//     carry 10k item_completed events and not one user_message), so the miner reads
//     the response items, which every version writes. Developer-role messages are
//     injected context; user-role messages that OPEN with an XML-ish tag
//     (`<environment_context>`, `<recommended_plugins>`, `<turn_aborted>`) are Codex's
//     own, not typed.
//   - response_item/function_call {name, call_id, arguments} and
//     function_call_output {call_id, output}; custom_tool_call {name, call_id} and
//     custom_tool_call_output likewise. The shell is `exec_command` / `shell` /
//     `local_shell` (function) and `exec` (custom). An output is sometimes a JSON
//     object wrapping the text under `output`; it is unwrapped before signature.
//
// A user message carries no uuid; `ordinal` is stable and unique within a rollout.
type codexRec struct {
	Timestamp string          `json:"timestamp"`
	Ordinal   int64           `json:"ordinal"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Output    string `json:"output"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

var codexInjectedRe = regexp.MustCompile(`^\s*<[a-z_]+>`)

func codexShell(name string) bool {
	switch name {
	case "exec_command", "shell", "local_shell", "exec":
		return true
	}
	return false
}

func readCodex(path string, start int64, o readOpts) (readResult, int64, error) {
	return readLog(path, start, o, codexShell, codexStep)
}

func codexStep(line string, c *convo) {
	var rec codexRec
	if json.Unmarshal([]byte(line), &rec) != nil {
		return
	}
	var p codexPayload
	if json.Unmarshal(rec.Payload, &p) != nil {
		return
	}
	at := parseTS(rec.Timestamp)
	switch rec.Type {
	case "session_meta":
		c.meta(p.SessionID, p.Cwd)
	case "response_item":
		switch p.Type {
		case "message":
			var texts []string
			for _, b := range p.Content {
				if strings.TrimSpace(b.Text) != "" {
					texts = append(texts, b.Text)
				}
			}
			text := strings.Join(texts, "\n")
			switch p.Role {
			case "assistant":
				c.assistant(text)
			case "user":
				if codexInjectedRe.MatchString(text) {
					return
				}
				c.human(text, at, "o"+strconv.FormatInt(rec.Ordinal, 10))
			}
		case "function_call", "custom_tool_call":
			c.toolCall(p.CallID, p.Name)
		case "function_call_output", "custom_tool_call_output":
			c.toolResult(p.CallID, codexOutputText(p.Output), at)
		}
	}
}

// codexOutputText unwraps a JSON-object output ({"output": "...", ...}) to its text;
// a plain string passes through.
func codexOutputText(s string) string {
	if !strings.HasPrefix(strings.TrimSpace(s), "{") {
		return s
	}
	var w struct {
		Output string `json:"output"`
	}
	if json.Unmarshal([]byte(s), &w) == nil && w.Output != "" {
		return w.Output
	}
	return s
}
