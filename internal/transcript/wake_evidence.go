package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
	// FirstTurnPrefix identifies a candidate that still needs independent
	// rotation evidence from its caller.
	FirstTurnPrefix bool
}

type wakePrefixMode uint8

const (
	wakeTailOnly wakePrefixMode = iota
	wakeStartupPrefix
	wakeFirstTurnPrefix
)

func CodexWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	return codexWakeWitness(session, turn, prompt, since, wakeTailOnly)
}

// CodexStartupWakeWitness is for an active, exact startup acknowledgement.
// It may read a second bounded range if the usual tail missed the turn start.
func CodexStartupWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	return codexWakeWitness(session, turn, prompt, since, wakeStartupPrefix)
}

// CodexFirstTurnWakeWitness tries the usual tail, then one original-rollout
// prefix. A prefix witness proves only the first native turn and input.
func CodexFirstTurnWakeWitness(session, turn, prompt string, since time.Time) WakeWitness {
	return codexWakeWitness(session, turn, prompt, since, wakeFirstTurnPrefix)
}

func codexWakeWitness(session, turn, prompt string, since time.Time, mode wakePrefixMode) WakeWitness {
	if session == "" || turn == "" || prompt == "" || since.IsZero() {
		return WakeWitness{}
	}
	var result WakeWitness
	var paths []string
	if mode == wakeFirstTurnPrefix {
		candidates := matchingCodexLogPaths(session, enumerateCodexCandidates())
		if !unambiguousWakeOriginal(session, candidates) {
			mode = wakeTailOnly
		}
		paths = foldCodexLogPaths(candidates)
	} else {
		paths = historyPaths("codex", session)
	}
	if len(paths) != 1 {
		mode = wakeTailOnly
	}
	for _, path := range paths {
		w := wakeWitnessInSession(path, session, turn, prompt, since, mode)
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

// Chat folds matching basenames for display. The workspace prefix needs one
// original rollout, so same-name paths may coexist only as aliases of one file.
func unambiguousWakeOriginal(session string, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	name := filepath.Base(paths[0])
	if base, _ := codexRolloutName(name, session); !base {
		return false
	}
	if len(paths) == 1 {
		return true
	}
	first, err := os.Stat(paths[0])
	if err != nil || !first.Mode().IsRegular() {
		return false
	}
	for _, path := range paths[1:] {
		if filepath.Base(path) != name {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || !os.SameFile(first, info) {
			return false
		}
	}
	return true
}

func wakeWitnessIn(path, turn, prompt string, since time.Time) WakeWitness {
	return wakeWitnessInSession(path, "", turn, prompt, since, wakeTailOnly)
}

const wakeWitnessBudget int64 = 256 << 10

type startupProofScope struct {
	turn      string
	cwd       string
	firstTurn bool
}

func wakeWitnessInSession(path, session, turn, prompt string, since time.Time, mode wakePrefixMode) WakeWitness {
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
	scope := startupProofScope{turn: turn, firstTurn: mode == wakeFirstTurnPrefix}
	if expectWake {
		var meta codexPayload
		if mode != wakeTailOnly {
			header := bufio.NewReader(io.NewSectionReader(f, 0, min(info.Size(), codexSessionMetaBudget)))
			row, err := header.ReadBytes('\n')
			if err != nil {
				return WakeWitness{}
			}
			meta = parseCodexSessionMeta(row)
			headerEnd = int64(len(row))
			if filepath.IsAbs(meta.Cwd) && !strings.ContainsAny(meta.Cwd, "\x00\r\n") {
				scope.cwd = meta.Cwd
			}
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
	if _, err := f.ReadAt(buf, offset); err != nil && (err != io.EOF || mode != wakeTailOnly) {
		return WakeWitness{}
	}
	if offset > 0 {
		_, tail, ok := bytes.Cut(buf, []byte{'\n'})
		if !ok {
			return WakeWitness{}
		}
		buf = tail
	}
	witness, missedStart := scanWakeWitness(buf, turn, prompt, since, expectWake, nil)
	if !witness.Delivered.IsZero() || !missedStart || headerEnd == 0 || offset < headerEnd {
		return witness
	}
	// Prove the complete start-to-input sequence inside one range. The tail
	// cannot fill a gap or supply a partial record from beyond the budget.
	prefix := make([]byte, min(wakeWitnessBudget, info.Size()-headerEnd))
	if _, err := f.ReadAt(prefix, headerEnd); err != nil {
		return WakeWitness{}
	}
	witness, _ = scanWakeWitness(prefix, turn, prompt, since, true, &scope)
	if !witness.Delivered.IsZero() {
		witness.StartupPrefix = mode == wakeStartupPrefix
		witness.FirstTurnPrefix = mode == wakeFirstTurnPrefix
	}
	return witness
}

// The second result allows a prefix read only when a usable tail never saw the
// exact turn start. A seen start with a rejected first input is conclusive.
func scanWakeWitness(buf []byte, turn, prompt string, since time.Time, expectWake bool, scope *startupProofScope) (WakeWitness, bool) {
	rows := bytes.Split(buf, []byte{'\n'})
	var start time.Time
	seenTurn := false
	usable := len(rows) > 1
	strict := scope != nil
	firstTurn := strict && scope.firstTurn
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
				if firstTurn && (seenTurn || boundary.Turn != turn) {
					return WakeWitness{}, false
				}
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
				if firstTurn {
					return WakeWitness{}, false
				}
				start = time.Time{}
				continue
			}
		}
		if start.IsZero() && !firstTurn {
			continue
		}
		if !strict && at.Before(start) {
			continue
		}
		raw, isPrompt, complete := wakePrompt(line, expectWake, scope)
		if !isPrompt {
			if firstTurn && !firstTurnSetup(line, turn) {
				return WakeWitness{}, false
			}
			continue
		}
		if !start.IsZero() && raw == prompt && (!expectWake || complete) && !at.Before(start) {
			return WakeWitness{Started: start, Delivered: at}, false
		}
		if firstTurn {
			return WakeWitness{}, false
		}
		// The first human-origin input establishes causation. A later copy
		// in a turn already opened by different input cannot attest the wake.
		start = time.Time{}
	}
	return WakeWitness{}, usable && !seenTurn
}

// The ordinary wake must precede execution in the successor. An assistant
// response or tool activity before the exact input makes a later echo unsafe
// to attribute, even when the earlier user input is unavailable.
func firstTurnSetup(line codexLine, turn string) bool {
	if len(line.Payload) == 0 || line.Payload[0] != '{' {
		return false
	}
	var p struct {
		Type string `json:"type"`
		Role string `json:"role"`
		Turn string `json:"turn_id"`
	}
	if json.Unmarshal(line.Payload, &p) != nil || (p.Turn != "" && p.Turn != turn) {
		return false
	}
	switch line.Type {
	case "world_state":
		return true
	case "turn_context":
		return p.Turn == turn
	case "event_msg":
		return p.Type == "thread_settings_applied"
	case "response_item":
		if p.Type == "additional_tools" {
			return p.Role == "developer"
		}
		// The caller already classified any user-role record's full content.
		return p.Type == "message" && (p.Role == "user" || p.Role == "developer" || p.Role == "system")
	default:
		return false
	}
}

func wakePrompt(line codexLine, expectWake bool, scope *startupProofScope) (text string, candidate, complete bool) {
	if line.Type != "event_msg" && line.Type != "response_item" {
		return "", false, false
	}
	var payload struct {
		codexPayload
		Turn string `json:"turn_id"`
	}
	if json.Unmarshal(line.Payload, &payload) != nil || (scope != nil && payload.Type == "") {
		return "", scope != nil, false
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
	case line.Type == "response_item" && payload.Type == "message":
		if payload.Role != "user" {
			switch payload.Role {
			case "assistant", "developer", "system", "tool":
				return "", false, false
			default:
				return "", scope != nil, false
			}
		}
		var parts []string
		contextOnly := scope != nil && len(payload.Content) > 0
		for _, item := range payload.Content {
			if item.Type == "input_text" && strings.TrimSpace(item.Text) != "" {
				parts = append(parts, item.Text)
			}
			hasMedia = hasMedia || item.Type == "input_image"
			contextOnly = contextOnly && item.Type == "input_text" && codexStartupContextBlock(item.Text, scope.cwd)
		}
		if contextOnly && (payload.Turn == "" || payload.Turn == scope.turn) {
			return "", false, false
		}
		text = strings.Join(parts, "\n")
		complete = len(payload.Content) == 1 && payload.Content[0].Type == "input_text"
	default:
		return "", false, false
	}
	if scope != nil {
		if !complete || strings.TrimSpace(text) == "" || payload.Turn != "" && payload.Turn != scope.turn {
			return text, true, false
		}
		// Modern records are classified block by block above. The legacy
		// event carries one plain text field.
		if line.Type == "event_msg" && codexStartupContextBlock(text, scope.cwd) {
			return "", false, false
		}
		return text, true, true
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

// Only a known harness block can be skipped in a continuous startup proof.
// A user-role record with another block may contain the actual first input.
func codexStartupContextBlock(text, cwd string) bool {
	text = strings.TrimSpace(text)
	firstLine, _, _ := strings.Cut(text, "\n")
	const heading = "# AGENTS.md instructions"
	return strings.TrimSpace(firstLine) == heading ||
		(cwd != "" && strings.TrimSuffix(firstLine, "\r") == heading+" for "+cwd) ||
		(strings.HasPrefix(text, "<environment_context>") && strings.HasSuffix(text, "</environment_context>"))
}
