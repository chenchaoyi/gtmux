package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

// WakeWitness joins an exact wake prompt to a newly started turn in the same
// rollout. A start next to a different prompt, or an old active turn reported
// by the receiver, cannot settle this wake. No transcript content escapes.
type WakeWitness struct {
	Started   time.Time
	Delivered time.Time
}

func CodexWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	if session == "" || turn == "" || prompt == "" || since.IsZero() {
		return WakeWitness{}
	}
	var result WakeWitness
	for _, path := range historyPaths("codex", session) {
		w := wakeWitnessIn(path, turn, prompt, since)
		if w.Delivered.IsZero() {
			continue
		}
		if !result.Delivered.IsZero() && result != w {
			return WakeWitness{} // divergent rollout evidence
		}
		result = w
	}
	return result
}

func wakeWitnessIn(path, turn, prompt string, since time.Time) WakeWitness {
	f, err := os.Open(path)
	if err != nil {
		return WakeWitness{}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return WakeWitness{}
	}
	const budget int64 = 256 << 10
	offset := max(0, info.Size()-budget)
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
		return WakeWitness{}
	}
	if offset > 0 {
		_, tail, ok := bytes.Cut(buf, []byte{'\n'})
		if !ok {
			return WakeWitness{}
		}
		buf = tail
	}
	rows := bytes.Split(buf, []byte{'\n'})
	var start time.Time
	// A partial final append is not evidence until its newline is durable.
	for _, row := range rows[:len(rows)-1] {
		var line codexLine
		if json.Unmarshal(row, &line) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
		if err != nil {
			continue
		}
		if line.Type == "event_msg" {
			var boundary struct {
				Type string `json:"type"`
				Turn string `json:"turn_id"`
			}
			if json.Unmarshal(line.Payload, &boundary) != nil {
				continue
			}
			switch boundary.Type {
			case "task_started":
				start = time.Time{}
				if boundary.Turn == turn && !at.Before(since) {
					start = at
				}
				continue
			case "task_complete", "turn_aborted":
				start = time.Time{}
				continue
			}
		}
		if start.IsZero() || at.Before(start) {
			continue
		}
		raw, isPrompt := wakePrompt(line)
		if !isPrompt {
			continue
		}
		if raw == prompt {
			return WakeWitness{Started: start, Delivered: at}
		}
		// The first human-origin input establishes causation. A later copy
		// in a turn already opened by different input cannot attest the wake.
		start = time.Time{}
	}
	return WakeWitness{}
}

func wakePrompt(line codexLine) (string, bool) {
	var payload codexPayload
	if json.Unmarshal(line.Payload, &payload) != nil {
		return "", false
	}
	var text string
	switch {
	case line.Type == "event_msg" && payload.Type == "user_message":
		text = payload.Message
	case line.Type == "response_item" && payload.Type == "message" && payload.Role == "user":
		var parts []string
		for _, item := range payload.Content {
			if item.Type == "input_text" && strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
		}
		text = strings.Join(parts, "\n")
	default:
		return "", false
	}
	_, real := codexUserPrompt(text)
	return text, real
}
