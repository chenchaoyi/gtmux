package driver

import (
	"encoding/json"
	"strings"
)

// HeadlessSpec is an agent's one-shot, non-interactive mode: how to build the
// launch argv (the goal travels as an ARGUMENT — no paste, no delivery
// verification needed) and how to read its structured output stream. The run's
// lifecycle truth (done / crash) comes from that stream plus the process exit
// code — never from screen classification.
type HeadlessSpec struct {
	// Args builds the one-shot argv for a goal (model optional, "" = default).
	Args func(goal, model string) []string
	// ParseLine folds one stdout line into the running outcome. TOLERANT by
	// contract: an unknown or unparsable line changes nothing (agent stream
	// formats drift across versions; the exit code is the backstop).
	ParseLine func(line string, o *HeadlessOutcome)
}

// HeadlessOutcome accumulates what a one-shot run's stream declared.
type HeadlessOutcome struct {
	Done    bool   // a terminal result event was seen on the stream
	Failed  bool   // …and it declared an error
	Summary string // the result/error text (for the completion event record)
	Session string // the run's session id, when the stream names one
}

// claudeHeadless: `claude -p <goal> --output-format stream-json` (stream-json
// requires --verbose). The stream is one JSON object per line; the terminal
// event is type:"result" carrying subtype/is_error/result and the session_id
// (also present on the type:"system" init event).
var claudeHeadless = &HeadlessSpec{
	Args: func(goal, model string) []string {
		args := []string{"claude", "-p", goal, "--output-format", "stream-json", "--verbose"}
		if model != "" {
			args = append(args, "--model", model)
		}
		return args
	},
	ParseLine: func(line string, o *HeadlessOutcome) {
		var ev struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			IsError   bool   `json:"is_error"`
			Result    string `json:"result"`
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			return
		}
		if ev.SessionID != "" {
			o.Session = ev.SessionID
		}
		if ev.Type != "result" {
			return
		}
		o.Done = true
		o.Failed = ev.IsError || (ev.Subtype != "" && ev.Subtype != "success")
		if s := strings.TrimSpace(ev.Result); s != "" {
			o.Summary = s
		} else if o.Failed && ev.Subtype != "" {
			o.Summary = ev.Subtype
		}
	},
}

// codexHeadless: `codex exec --json <goal>`. Events arrive as JSON lines whose
// type may sit flat or nested under msg (protocol versions differ); the
// terminal events are task_complete (with last_agent_message) and error.
var codexHeadless = &HeadlessSpec{
	Args: func(goal, model string) []string {
		args := []string{"codex", "exec", "--json"}
		if model != "" {
			args = append(args, "-m", model)
		}
		return append(args, goal)
	},
	ParseLine: func(line string, o *HeadlessOutcome) {
		var ev struct {
			Type string `json:"type"`
			Msg  struct {
				Type             string `json:"type"`
				LastAgentMessage string `json:"last_agent_message"`
				Message          string `json:"message"`
			} `json:"msg"`
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			return
		}
		if ev.SessionID != "" {
			o.Session = ev.SessionID
		}
		typ := ev.Type
		if ev.Msg.Type != "" {
			typ = ev.Msg.Type
		}
		switch typ {
		case "task_complete":
			o.Done = true
			if s := strings.TrimSpace(ev.Msg.LastAgentMessage); s != "" {
				o.Summary = s
			}
		case "error":
			o.Done, o.Failed = true, true
			if s := strings.TrimSpace(ev.Msg.Message); s != "" {
				o.Summary = s
			}
		}
	},
}
