package transcript

import (
	"encoding/json"
	"strings"
	"time"
)

// codexActivityTimeIn reads top-level stamps only. Codex can append settings and
// quota records to an otherwise idle rollout weeks after its last turn ended.
// Unknown or partial records provide no evidence of conversation activity.
func codexActivityTimeIn(buf string) int64 {
	lines := strings.Split(buf, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var line codexLine
		if json.Unmarshal([]byte(lines[i]), &line) != nil {
			continue
		}
		var payload struct {
			Type string `json:"type"`
			Role string `json:"role"`
		}
		if json.Unmarshal(line.Payload, &payload) != nil {
			continue
		}
		activity := false
		switch line.Type {
		case "event_msg":
			switch payload.Type {
			case "user_message", "agent_message", "task_started", "task_complete", "turn_aborted":
				activity = true
			}
		case "response_item":
			activity = payload.Type == "message" && (payload.Role == "user" || payload.Role == "assistant")
		}
		if activity {
			if at, err := time.Parse(time.RFC3339Nano, line.Timestamp); err == nil && at.Unix() > 0 {
				return at.Unix()
			}
		}
	}
	return 0
}
