package mine

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
)

// Kimi Code journals a session as $KIMI_CODE_HOME/sessions/<workDirKey>/<id>/agents/
// main/wire.jsonl: {"type": …, …payload at the top level…, "time": <epoch ms>}. The
// conversation is two record types (see internal/transcript/kimi.go, verified against
// kimi 0.41.0): `context.append_message` carries the user's prompt (role user, and only
// an absent or `user` origin — every other origin is an injection, a hook result, a
// compaction summary…) and sometimes an assistant message; `context.append_loop_event`
// with `content.part` carries the assistant's streamed reply. `partial` messages are
// re-appended in final form and skipped.
//
// Tool RESULTS were not observed as a journal record on a real session when this was
// written, so this reader claims correction leads only; the shell predicate never
// matches. When a result record is observed and pinned, add it here — not before.
//
// A message has no id; the epoch-ms time plus a running index within the file is
// stable enough (a pass resumes at a line boundary, so the index restarts — the time
// keeps the pair unique).
type kimiRec struct {
	Type    string `json:"type"`
	Time    int64  `json:"time"`
	Message *struct {
		Role    string          `json:"role"`
		Origin  json.RawMessage `json:"origin"`
		Partial bool            `json:"partial"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	Event *struct {
		Type string `json:"type"`
		Part *struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"part"`
	} `json:"event"`
}

func readKimi(path string, start int64, o readOpts) (readResult, int64, error) {
	// …/sessions/<workDirKey>/<sessionId>/agents/main/wire.jsonl
	session := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	n := 0
	return readLog(path, start, o, func(string) bool { return false }, func(line string, c *convo) {
		c.meta(session, "")
		var r kimiRec
		if json.Unmarshal([]byte(line), &r) != nil {
			return
		}
		n++
		switch r.Type {
		case "context.append_loop_event":
			if r.Event != nil && r.Event.Type == "content.part" && r.Event.Part != nil && r.Event.Part.Type == "text" {
				c.assistant(r.Event.Part.Text)
			}
		case "context.append_message":
			if r.Message == nil || r.Message.Partial {
				return
			}
			var texts []string
			for _, p := range r.Message.Content {
				if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
					texts = append(texts, p.Text)
				}
			}
			text := strings.Join(texts, "\n")
			switch r.Message.Role {
			case "assistant":
				c.assistant(text)
			case "user":
				if k := kimiOrigin(r.Message.Origin); k != "" && k != "user" {
					return
				}
				c.human(text, r.Time/1000, strconv.FormatInt(r.Time, 10)+"/"+strconv.Itoa(n))
			}
		}
	})
}

// kimiOrigin reads the origin whichever way it is written — the object kimi 0.41.0
// emits (`{"kind":"user"}`) or the bare string its manifest documents.
func kimiOrigin(raw json.RawMessage) string {
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
