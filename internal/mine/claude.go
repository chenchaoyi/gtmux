package mine

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// Claude Code writes one JSON record per line under ~/.claude/projects/<cwd-slug>/
// <sessionId>.jsonl. The fields read here are the same ones internal/transcript trusts
// (see its schema notes); the reader is separate because it needs what the chat view
// does not — record uuids for stable ids, tool_result bodies for error signatures, and
// a resumable byte offset that lands on a line boundary.
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
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

type readOpts struct {
	sinceUnix int64
	machine   map[string]bool
	// carry is the conversational state at the offset a pass resumes from: whether the
	// assistant had spoken since the last human line, the tail of what it said, and the
	// tool_use ids still waiting for a result. Without it a pass that resumes right after
	// an assistant reply cannot see that the next human line answers one.
	carry carryState
}

// carryState is persisted per file in the ledger.
type carryState struct {
	Spoke bool              `json:"spoke,omitempty"`
	Tail  string            `json:"tail,omitempty"`
	Tools map[string]string `json:"tools,omitempty"` // tool_use id → name, results pending
}

type errorHit struct {
	line    string
	count   int
	at      int64
	session string
}

type readResult struct {
	humanLines  int
	corrections []Candidate
	errors      map[string]errorHit
	carry       carryState
}

// Bounds. A typed line longer than pasteRunes is a paste, not a reaction; the clips keep
// a spool line readable.
const (
	pasteRunes   = 1500
	lineRunes    = 300
	contextRunes = 240
)

// readClaude parses from start to the last COMPLETE line and returns the offset after
// it — a trailing partial line (the agent mid-write) is left for the next pass.
func readClaude(path string, start int64, o readOpts) (res readResult, end int64, err error) {
	res = readResult{errors: map[string]errorHit{}}
	f, err := os.Open(path)
	if err != nil {
		return res, start, err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return res, start, err
	}
	// Named results on purpose: the deferred carry snapshot below must land in what the
	// caller receives, not in a copy made before the defer ran.
	r := bufio.NewReaderSize(f, 256*1024)
	off := start
	lastAssistant, assistantSpoke := o.carry.Tail, o.carry.Spoke
	toolNames := map[string]string{} // tool_use id → name
	for id, name := range o.carry.Tools {
		toolNames[id] = name
	}
	var session, cwd string
	defer func() {
		res.carry = carryState{Spoke: assistantSpoke, Tail: tailRunes(lastAssistant, contextRunes), Tools: pendingTools(toolNames)}
	}()
	for {
		line, rerr := r.ReadString('\n')
		if rerr != nil && rerr != io.EOF {
			return res, off, rerr
		}
		if rerr == io.EOF && !strings.HasSuffix(line, "\n") {
			break // partial tail — not consumed
		}
		if len(line) == 0 {
			break
		}
		off += int64(len(line))
		var rec claudeRec
		if json.Unmarshal([]byte(line), &rec) != nil || rec.Message == nil || rec.IsSidechain {
			continue
		}
		if rec.SessionID != "" {
			session = rec.SessionID
		}
		if rec.CWD != "" {
			cwd = rec.CWD
		}
		switch rec.Type {
		case "assistant":
			text, uses := assistantParts(rec.Message.Content)
			for id, name := range uses {
				toolNames[id] = name
			}
			if text != "" {
				lastAssistant = text
				assistantSpoke = true
			}
		case "user":
			if rec.IsMeta || rec.IsCompactSummary {
				continue
			}
			if results, ok := toolResults(rec.Message.Content); ok {
				for _, tr := range results {
					name := toolNames[tr.ToolUseID]
					delete(toolNames, tr.ToolUseID)
					if name != "Bash" {
						continue
					}
					sig, ok := errorSignature(blockText(tr.Content))
					if !ok {
						continue
					}
					h := res.errors[sig]
					h.line, h.session = sig, session
					h.count++
					h.at = maxInt64(h.at, parseTS(rec.Timestamp))
					res.errors[sig] = h
				}
				continue
			}
			text, kind := transcript.ClassifyUserPrompt(promptText(rec.Message.Content))
			if kind != transcript.PromptUser {
				continue
			}
			spoke := assistantSpoke
			assistantSpoke = false
			if o.machine[HeadKey(text)] {
				continue // the machine typed this (gtmux send / wake) — not the human
			}
			if utf8.RuneCountInString(text) > pasteRunes {
				continue
			}
			res.humanLines++
			at := parseTS(rec.Timestamp)
			if at < o.sinceUnix || !spoke || !isCorrectionShaped(text) {
				continue
			}
			res.corrections = append(res.corrections, Candidate{
				Kind: KindCorrection, ID: candidateID(session, rec.UUID), At: at,
				Session: session, Project: filepath.Base(cwd),
				Line:    clipRunes(strings.Join(strings.Fields(text), " "), lineRunes),
				Context: tailRunes(strings.Join(strings.Fields(lastAssistant), " "), contextRunes),
			})
		}
		if rerr == io.EOF {
			break
		}
	}
	return res, off, nil
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

// blockText flattens a tool_result payload (string | block[]) to text.
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

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func tailRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return "…" + string(r[len(r)-n:])
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

// pendingTools keeps the tool_use ids whose result has not arrived, bounded — a
// runaway map would only mean a log that never answers its tool calls.
func pendingTools(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := map[string]string{}
	for id, name := range m {
		if len(out) >= 16 {
			break
		}
		out[id] = name
	}
	return out
}
