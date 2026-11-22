package transcript

import (
	"bufio"
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
	Started       time.Time
	Delivered     time.Time
	StartupPrefix bool
}

func CodexWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	return codexWakeWitness(session, turn, prompt, since, false)
}

// CodexStartupWakeWitness is for an active, exact startup acknowledgement.
// It may read a second bounded range if the usual tail missed the turn start.
func CodexStartupWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	return codexWakeWitness(session, turn, prompt, since, true)
}

func codexWakeWitness(session, turn, prompt string, since time.Time, startup bool) WakeWitness {
	if session == "" || turn == "" || prompt == "" || since.IsZero() {
		return WakeWitness{}
	}
	var result WakeWitness
	paths := historyPaths("codex", session)
	for _, path := range paths {
		w := wakeWitnessInSession(path, session, turn, prompt, since, startup && len(paths) == 1)
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
	return wakeWitnessInSession(path, "", turn, prompt, since, false)
}

const wakeWitnessBudget int64 = 256 << 10

func wakeWitnessInSession(path, session, turn, prompt string, since time.Time, startup bool) WakeWitness {
	f, err := os.Open(path)
	if err != nil {
		return WakeWitness{}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return WakeWitness{}
	}
	expectWake := strings.HasPrefix(prompt, Sigil+" ")
	var headerEnd int64
	if expectWake {
		var meta codexPayload
		if startup {
			header := bufio.NewReader(io.NewSectionReader(f, 0, min(info.Size(), codexSessionMetaBudget)))
			row, err := header.ReadBytes('\n')
			if err != nil {
				return WakeWitness{}
			}
			meta = parseCodexSessionMeta(row)
			headerEnd = int64(len(row))
		} else {
			meta = scanCodexSessionMeta(f)
		}
		id := meta.ID
		if id == "" {
			id = meta.SessionID
		}
		if session == "" || id != session {
			return WakeWitness{}
		}
		if meta.SessionID != "" && meta.SessionID != session {
			headerEnd = 0 // Never extend an ambiguous metadata identity.
		}
	}
	offset := max(0, info.Size()-wakeWitnessBudget)
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && (err != io.EOF || startup) {
		return WakeWitness{}
	}
	if offset > 0 {
		_, tail, ok := bytes.Cut(buf, []byte{'\n'})
		if !ok {
			return WakeWitness{}
		}
		buf = tail
	}
	witness, missedStart := scanWakeWitness(buf, turn, prompt, since, expectWake, false)
	if !witness.Delivered.IsZero() || !missedStart || headerEnd == 0 || offset < headerEnd {
		return witness
	}
	// Prove the complete start-to-input sequence inside one range. The tail
	// cannot fill a gap or supply a partial record from beyond the budget.
	prefix := make([]byte, min(wakeWitnessBudget, info.Size()-headerEnd))
	if _, err := f.ReadAt(prefix, headerEnd); err != nil {
		return WakeWitness{}
	}
	witness, _ = scanWakeWitness(prefix, turn, prompt, since, true, true)
	witness.StartupPrefix = !witness.Delivered.IsZero()
	return witness
}

// The second result allows a prefix read only when a usable tail never saw the
// exact turn start. A seen start with a rejected first input is conclusive.
func scanWakeWitness(buf []byte, turn, prompt string, since time.Time, expectWake, strict bool) (WakeWitness, bool) {
	rows := bytes.Split(buf, []byte{'\n'})
	var start time.Time
	seenTurn := false
	usable := len(rows) > 1
	// A partial final append is not evidence until its newline is durable.
	for _, row := range rows[:len(rows)-1] {
		var line codexLine
		if json.Unmarshal(row, &line) != nil || line.Type == "" {
			if strict {
				return WakeWitness{}, false
			}
			usable = false
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, line.Timestamp)
		if err != nil {
			if strict {
				return WakeWitness{}, false
			}
			usable = false
			continue
		}
		if line.Type == "event_msg" {
			var boundary struct {
				Type string `json:"type"`
				Turn string `json:"turn_id"`
			}
			if json.Unmarshal(line.Payload, &boundary) != nil || (strict && boundary.Type == "") {
				if strict {
					return WakeWitness{}, false
				}
				usable = false
				continue
			}
			switch boundary.Type {
			case "task_started":
				start = time.Time{}
				if boundary.Turn == turn {
					if at.Before(since) || seenTurn {
						return WakeWitness{}, false
					}
					seenTurn = true
					start = at
				}
				continue
			case "task_complete", "turn_aborted":
				start = time.Time{}
				continue
			}
		}
		if start.IsZero() {
			continue
		}
		proofTurn := ""
		if strict {
			proofTurn = turn
		} else if at.Before(start) {
			continue
		}
		raw, isPrompt, complete := wakePrompt(line, expectWake, proofTurn)
		if !isPrompt {
			continue
		}
		if raw == prompt && (!expectWake || complete) && !at.Before(start) {
			return WakeWitness{Started: start, Delivered: at}, false
		}
		// The first human-origin input establishes causation. A later copy
		// in a turn already opened by different input cannot attest the wake.
		start = time.Time{}
	}
	return WakeWitness{}, usable && !seenTurn
}

func wakePrompt(line codexLine, expectWake bool, proofTurn string) (text string, candidate, complete bool) {
	if line.Type != "event_msg" && line.Type != "response_item" {
		return "", false, false
	}
	var payload struct {
		codexPayload
		Turn string `json:"turn_id"`
	}
	if json.Unmarshal(line.Payload, &payload) != nil || (proofTurn != "" && payload.Type == "") {
		return "", proofTurn != "", false
	}
	hasMedia := false
	switch {
	case line.Type == "event_msg" && payload.Type == "user_message":
		text = payload.Message
		var extras struct {
			Images      []json.RawMessage `json:"images"`
			LocalImages []json.RawMessage `json:"local_images"`
			Elements    []json.RawMessage `json:"text_elements"`
		}
		parsed := json.Unmarshal(line.Payload, &extras) == nil
		hasMedia = len(extras.Images)+len(extras.LocalImages) > 0
		complete = parsed && !hasMedia && len(extras.Elements) == 0
	case line.Type == "response_item" && payload.Type == "message" && payload.Role == "user":
		var parts []string
		for _, item := range payload.Content {
			if item.Type == "input_text" && strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
			hasMedia = hasMedia || item.Type == "input_image"
		}
		text = strings.Join(parts, "\n")
		complete = len(payload.Content) == 1 && payload.Content[0].Type == "input_text"
	default:
		return "", false, false
	}
	if proofTurn != "" && (!complete || strings.TrimSpace(text) == "" || payload.Turn != "" && payload.Turn != proofTurn) {
		return text, true, false
	}
	// This observer already has a bound receiver receipt and expects exact raw
	// text. The chat filter intentionally strips wake echoes. A different wake
	// as the first input still invalidates this turn as proof of ours.
	if expectWake && (hasMedia || strings.HasPrefix(strings.TrimSpace(text), Sigil+" ")) {
		return text, true, complete
	}
	_, real := codexUserPrompt(text)
	return text, real, complete
}
