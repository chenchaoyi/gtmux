package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
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
		if json.Unmarshal(row, &line) != nil || line.Type != "event_msg" {
			continue
		}
		var e struct {
			Type    string `json:"type"`
			Turn    string `json:"turn_id"`
			Message string `json:"message"`
		}
		if json.Unmarshal(line.Payload, &e) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
		if err != nil {
			continue
		}
		switch e.Type {
		case "task_started":
			start = time.Time{}
			if e.Turn == turn && !at.Before(since) {
				start = at
			}
		case "task_complete", "turn_aborted":
			start = time.Time{}
		case "user_message":
			if !start.IsZero() && !at.Before(start) && e.Message == prompt {
				return WakeWitness{Started: start, Delivered: at}
			}
			// The first input establishes the turn's causation. A subsequent
			// identical line on a different user-started turn cannot attest it.
			start = time.Time{}
		}
	}
	return WakeWitness{}
}
