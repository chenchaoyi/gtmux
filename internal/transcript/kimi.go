package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Kimi Code keeps an EVENT-SOURCED journal, not a message log: one session is a
// directory under $KIMI_CODE_HOME/sessions/<workDirKey>/<sessionId>/, and its
// conversation is `agents/main/wire.jsonl` — sub-agents get their own agents/<id>/.
//
// Each line is {"type": …, …payload spread at the top level…, "time": <epoch ms>},
// after an opening {"type":"metadata","protocol_version",…}. Sixty record types are
// journaled (token counts, permission decisions, plan revisions, compaction, cron);
// exactly one carries the conversation, `context.append_message`, and everything else
// is skipped rather than guessed at.
//
// Two fields of that record decide what a turn is, and both matter:
//
//   - `origin` separates the USER's instruction from the eleven other ways a user-role
//     message gets appended — skill_activation, injection, system_trigger, hook_result,
//     compaction_summary, task, cron_job, retry, shell_command, plugin_command. Only an
//     absent origin or `"user"` opens a turn. Without that filter the digest's `goal`
//     would eventually report gtmux's own hook output back to gtmux as the thing the
//     user asked for. Verified against a real journal: a turn's own prompt carries
//     `{"kind":"user"}` and the date/system reminders beside it carry
//     `{"kind":"injection"}`.
//
//     Its SHAPE is read leniently, and that is not defensiveness. The generated wire
//     manifest declares `origin` a string; kimi 0.41.0 actually writes an OBJECT,
//     `{"kind":"user"}`. Decoding it into a string made json.Unmarshal fail, which made
//     kimiStep drop the record, which made a real session parse to ZERO turns — while
//     thirteen unit tests built from the manifest passed. So both shapes are accepted.
//   - `partial` marks a message still streaming. Its final form is appended again, so
//     taking both would print the reply twice, the first copy truncated.
//
// The two halves of a turn are journaled in DIFFERENT records, which the manifest does
// not make obvious and a real session does: the user's prompt is a
// `context.append_message`, but the assistant's reply is a `context.append_loop_event`
// carrying `content.part`. A completed headless turn writes NO assistant
// `context.append_message` at all — reading only that record type gave a turn with a
// prompt and an empty response.
//
// Those parts are already COALESCED. A reply streamed as three deltas is journaled as
// one part holding the finished text (measured), so each part is appended as it stands
// rather than accumulated — accumulating would print the tail of the reply twice.
//
// Verified against kimi 0.41.0 (agent-core-v2, wire protocol 1.5) by running a real
// session against a local stand-in provider.

// kimiHome is Kimi's data root. KIMI_CODE_HOME relocates all of it — config,
// sessions, credentials — so it is the only override there is.
func kimiHome() string {
	if h := os.Getenv("KIMI_CODE_HOME"); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".kimi-code")
}

// kimiIndexLine is one record of session_index.jsonl: an entry, or a deletion.
type kimiIndexLine struct {
	SessionID  string `json:"sessionId"`
	SessionDir string `json:"sessionDir"`
	WorkDir    string `json:"workDir"`
	Deleted    bool   `json:"deleted"`
}

// kimiLogPath maps a session id to its main agent's journal.
//
// The index is consulted first because it is authoritative: it carries the session's
// real directory, which is keyed by a HASH of the working directory that gtmux has no
// business re-deriving. It is append-only with tombstones, so the LAST record for an
// id wins — an id that was deleted and its directory reused would otherwise resolve to
// a stale path. A glob is the fallback for an index that is missing or truncated.
func kimiLogPath(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	if dir := kimiSessionDir(sessionID); dir != "" {
		if p := filepath.Join(dir, "agents", "main", "wire.jsonl"); fileExists(p) {
			return p
		}
	}
	m, _ := filepath.Glob(filepath.Join(kimiHome(), "sessions", "*", sessionID, "agents", "main", "wire.jsonl"))
	if len(m) > 0 {
		return m[0]
	}
	return ""
}

// kimiSessionDir reads the session index for one id, honouring tombstones.
func kimiSessionDir(sessionID string) string {
	f, err := os.Open(filepath.Join(kimiHome(), "session_index.jsonl"))
	if err != nil {
		return ""
	}
	defer f.Close()
	dir := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var e kimiIndexLine
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.SessionID != sessionID {
			continue
		}
		if e.Deleted {
			dir = "" // a later entry may re-create it; the last record wins
			continue
		}
		dir = e.SessionDir
	}
	return dir
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// kimiWire is one journal line. Only the fields gtmux reads are declared; the other
// fifty-nine record types unmarshal into a Type that no case matches.
type kimiWire struct {
	Type    string `json:"type"`
	Time    int64  `json:"time"` // epoch ms
	Message *struct {
		Role    string          `json:"role"`
		Origin  json.RawMessage `json:"origin"`
		Partial bool            `json:"partial"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		ToolCalls []struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"toolCalls"`
	} `json:"message"`
	// The assistant side of a turn: `context.append_loop_event` with a `content.part`.
	Event *struct {
		Type string `json:"type"`
		Part *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"part"`
	} `json:"event"`
}

// kimiText joins the text parts of a message, dropping everything else.
//
// `think` parts are the model's reasoning and are deliberately NOT included: the
// digest's job is to report what the agent SAID, and a chain of thought read as the
// answer is worse than no answer. Image/audio/video parts have no text to add.
func (w *kimiWire) text() string {
	var b []string
	for _, p := range w.Message.Content {
		if p.Type == "text" {
			if t := strings.TrimSpace(p.Text); t != "" {
				b = append(b, t)
			}
		}
	}
	return strings.Join(b, "\n")
}

// kimiStep feeds one journal line to the parser.
func kimiStep(line string, st *parseState) {
	var w kimiWire
	if json.Unmarshal([]byte(line), &w) != nil {
		return
	}
	if w.Type == kimiLoopRecord {
		if w.Event != nil && w.Event.Type == "content.part" && w.Event.Part != nil && w.Event.Part.Type == "text" {
			if t := strings.TrimSpace(w.Event.Part.Text); t != "" {
				st.addText(t)
			}
		}
		return
	}
	if w.Type != kimiMessageRecord || w.Message == nil {
		return
	}
	if w.Message.Partial {
		return // its finished form is appended again
	}
	switch w.Message.Role {
	case "user":
		// Only a real instruction opens a turn — see the origin note above.
		if o := originKind(w.Message.Origin); o != "" && o != "user" {
			return
		}
		// TrimSpace only. Claude's CleanUserPrompt exists to guess which user-role
		// messages were injected by the harness; `origin` states it outright, so
		// running the heuristic on top could only drop a real prompt.
		if t := strings.TrimSpace(w.text()); t != "" {
			st.open(t, kimiTime(w.Time))
		}
	case "assistant":
		if t := w.text(); t != "" {
			st.addText(t)
		}
		var steps []Step
		for _, c := range w.Message.ToolCalls {
			if c.Name == "" {
				continue
			}
			steps = append(steps, Step{Kind: "tool", Title: kimiToolName(c.Name), Detail: kimiToolDetail(c.Arguments)})
		}
		st.addSteps(steps)
	}
}

// originKind reads the origin whichever way it is written: the object kimi 0.41.0
// actually emits (`{"kind":"user"}`) or the bare string its own manifest documents.
// Anything else reads as absent, which lets the message through — the filter exists to
// drop known non-user origins, not to guess at unknown ones.
func originKind(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &o) == nil {
		return o.Kind
	}
	return ""
}

// kimiTime renders the journal's epoch-ms stamp as RFC3339, the format every other
// parser hands up. A missing or zero time yields "" rather than 1970.
func kimiTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Format(time.RFC3339)
}

// kimiToolName is the tool as a reader knows it. Kimi's names are already the plain
// ones (Bash, Read, Edit); an MCP tool arrives dotted, and its last segment is the
// part that says what it did.
func kimiToolName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

// kimiToolDetail pulls the one argument worth showing beside a tool step. Arguments
// arrive as a JSON *string* (or null), so a malformed or absent one is normal and
// yields no detail rather than an error.
func kimiToolDetail(args string) string {
	if strings.TrimSpace(args) == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(args), &m) != nil {
		return ""
	}
	for _, k := range []string{"command", "file_path", "path", "pattern", "query", "url", "description"} {
		if v, ok := m[k].(string); ok {
			if v = strings.TrimSpace(v); v != "" {
				return firstLine(v)
			}
		}
	}
	return ""
}

// firstLine keeps a multi-line command from becoming a multi-line step label.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
