package mine

import (
	"encoding/json"
	"strings"
	"time"
)

// Claude Code writes one JSON record per line under ~/.claude/projects/<cwd-slug>/
// <sessionId>.jsonl. The fields read here are the same ones internal/transcript trusts
// (see its schema notes); the reader is separate because it needs what the chat view
// does not — record uuids for stable ids and tool_result bodies for error signatures.
type claudeRec struct {
	Type             string `json:"type"`
	UUID             string `json:"uuid"`
	SessionID        string `json:"sessionId"`
	CWD              string `json:"cwd"`
	Timestamp        string `json:"timestamp"`
	IsMeta           bool   `json:"isMeta"`
	IsSidechain      bool   `json:"isSidechain"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Message          *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

func claudeShell(name string) bool { return name == "Bash" }

func readClaude(path string, start int64, o readOpts) (readResult, int64, error) {
	return readLog(path, start, o, claudeShell, claudeStep)
}

func claudeStep(line string, c *convo) {
	var rec claudeRec
	if json.Unmarshal([]byte(line), &rec) != nil || rec.Message == nil || rec.IsSidechain {
		return
	}
	c.meta(rec.SessionID, rec.CWD)
	switch rec.Type {
	case "assistant":
		text, uses := assistantParts(rec.Message.Content)
		for id, name := range uses {
			c.toolCall(id, name)
		}
		c.assistant(text)
	case "user":
		if rec.IsMeta || rec.IsCompactSummary {
			return
		}
		if results, ok := toolResults(rec.Message.Content); ok {
			for _, tr := range results {
				c.toolResult(tr.ToolUseID, blockText(tr.Content), parseTS(rec.Timestamp))
			}
			return
		}
		c.human(promptText(rec.Message.Content), parseTS(rec.Timestamp), rec.UUID)
	}
}

// assistantParts returns the concatenated text blocks and the tool_use id→name map.
func assistantParts(raw json.RawMessage) (string, map[string]string) {
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s), nil
		}
		return "", nil
	}
	var texts []string
	uses := map[string]string{}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" {
				texts = append(texts, t)
			}
		case "tool_use":
			uses[b.ID] = b.Name
		}
	}
	return strings.Join(texts, "\n"), uses
}

// toolResults returns the tool_result blocks when the content is a tool-result turn.
func toolResults(raw json.RawMessage) ([]claudeBlock, bool) {
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return nil, false
	}
	var out []claudeBlock
	for _, b := range blocks {
		if b.Type == "tool_result" {
			out = append(out, b)
		}
	}
	return out, len(out) > 0
}

// promptText flattens typed content (string or text blocks) to one string.
func promptText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return blockText(raw)
}

// blockText flattens a string-or-text-blocks payload to text.
func blockText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func parseTS(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// HeadsFromSendSummaries builds the machine-head set from `gtmux:audit:send` summaries
// ("<state>: <payload head>"). Wake lines need no entry — they open with the gtmux sigil
// and ClassifyUserPrompt already drops them.
func HeadsFromSendSummaries(summaries []string) map[string]bool {
	heads := map[string]bool{}
	for _, s := range summaries {
		if _, payload, ok := strings.Cut(s, ": "); ok {
			s = payload
		}
		if k := HeadKey(s); k != "" {
			heads[k] = true
		}
	}
	return heads
}
