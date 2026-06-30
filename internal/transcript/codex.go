package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Codex logs live at $CODEX_HOME/sessions/YYYY/MM/DD/rollout-<ts>-<sessionId>.jsonl
// (default CODEX_HOME=~/.codex), with archived_sessions/ as a fallback. Each line
// is {timestamp, type, payload}.
//
// Schema notes (from the 2026-06 prior-art survey, see docs/design/RESEARCH-…):
//   - the `event_msg` stream gives CLEAN text: user_message / agent_message
//     (display-only, no synthetic developer/system noise) — perfect for a glance.
//   - tool calls live in `response_item` as function_call (name + arguments as a
//     JSON *string*); these interleave with event_msg in file order, so a single
//     linear pass keeps steps in the right place.
//   - the FINAL answer is task_complete.last_agent_message (authoritative), else
//     the latest agent_message.
//   - response_item messages with role=="developer" are injected context — ignored.
//   - token_count events are everywhere and irrelevant to turn structure — ignored.

type codexLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type             string `json:"type"`
	Role             string `json:"role"`
	Message          string `json:"message"`            // event_msg user_message/agent_message
	LastAgentMessage string `json:"last_agent_message"` // event_msg task_complete
	Name             string `json:"name"`               // response_item function_call
	Arguments        string `json:"arguments"`          // response_item function_call (JSON string)
}

func codexHome() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".codex")
}

func codexLogPath(sessionID string) string {
	home := codexHome()
	for _, pat := range []string{
		filepath.Join(home, "sessions", "*", "*", "*", "rollout-*-"+sessionID+".jsonl"),
		filepath.Join(home, "archived_sessions", "rollout-*-"+sessionID+".jsonl"),
	} {
		if m, _ := filepath.Glob(pat); len(m) > 0 {
			return m[0]
		}
	}
	return ""
}

// codexStep folds one Codex log line into the parse state: event_msg user_message
// opens a turn; agent_message / task_complete set the (latest/authoritative)
// reply; response_item function_call adds a tool step.
func codexStep(line string, st *parseState) {
	var e codexLine
	if json.Unmarshal([]byte(line), &e) != nil || len(e.Payload) == 0 {
		return
	}
	var p codexPayload
	if json.Unmarshal(e.Payload, &p) != nil {
		return
	}
	switch e.Type {
	case "event_msg":
		switch p.Type {
		case "user_message":
			if strings.TrimSpace(p.Message) == "" {
				return
			}
			st.open(strings.TrimSpace(p.Message), e.Timestamp)
		case "agent_message":
			if msg := strings.TrimSpace(p.Message); msg != "" {
				st.addText(msg) // each agent message starts a new bubble
			}
		case "task_complete":
			// authoritative final reply — append it unless an agent_message already
			// carried it (avoid duplicating the closing paragraph).
			if fin := strings.TrimSpace(p.LastAgentMessage); fin != "" && !lastSegmentText(st, fin) {
				st.addText(fin)
			}
		}
	case "response_item":
		if p.Type == "function_call" && p.Name != "" {
			st.addSteps([]Step{{Kind: "tool", Title: codexToolName(p.Name), Detail: codexToolDetail(p.Arguments)}})
		}
	}
}

// codexToolName tidies a raw function name (exec_command → "exec") for display.
func codexToolName(name string) string {
	switch name {
	case "exec_command", "shell", "local_shell":
		return "exec"
	case "apply_patch":
		return "patch"
	}
	return name
}

// codexToolDetail pulls a short summary out of a function_call's JSON-string args
// (cmd / command / path / workdir).
func codexToolDetail(args string) string {
	if strings.TrimSpace(args) == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(args), &m) != nil {
		return clip(args, 80)
	}
	for _, k := range []string{"cmd", "command", "file_path", "path", "query", "workdir"} {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return clip(t, 80)
				}
			case []any:
				var parts []string
				for _, x := range t {
					if s, ok := x.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					return clip(strings.Join(parts, " "), 80)
				}
			}
		}
	}
	return ""
}
