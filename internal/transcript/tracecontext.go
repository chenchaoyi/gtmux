package transcript

import "strings"

// ContextRecord is an explicitly recorded harness input, never reasoning, a tool
// result, or a reconstructed model request. It does not become a user prompt.
type ContextRecord struct {
	Title        string `json:"title"`
	Text         string `json:"text"`
	Source       string `json:"source"`
	Time         string `json:"time,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	BeforePrompt bool   `json:"before_prompt,omitempty"`
}

const maxContextBytes = 8 << 10
const maxTurnContextBytes = 64 << 10
const maxTurnContexts = 32

func (st *parseState) addContext(title, text, source, ts string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	count, size := 0, 0
	for _, value := range st.pendingContexts {
		count++
		size += len(value.Text)
	}
	if st.cur != nil {
		for _, segment := range st.cur.Segments {
			if segment.Context != nil {
				count++
				size += len(segment.Context.Text)
			}
		}
	}
	if count >= maxTurnContexts || size >= maxTurnContextBytes {
		return
	}
	limit := min(maxContextBytes, maxTurnContextBytes-size)
	body, cut := boundedToolText(text, limit)
	// Codex may mirror the same user input in response_item and event_msg.
	// Compare only the uninterrupted context prefix; repeated later inputs remain
	// distinct and the first observed origin remains the provenance.
	if strings.HasPrefix(source, "codex/") {
		mirrored := func(previous ContextRecord) bool {
			return previous.Source != source && strings.HasPrefix(previous.Source, "codex/") && previous.Title == title && previous.Text == body
		}
		for _, previous := range st.pendingContexts {
			if mirrored(previous) {
				return
			}
		}
		if st.cur != nil {
			for i := len(st.cur.Segments) - 1; i >= 0; i-- {
				segment := st.cur.Segments[i]
				if segment.Context == nil || segment.Text != "" || len(segment.Steps) > 0 {
					break
				}
				if mirrored(*segment.Context) {
					return
				}
			}
		}
	}
	value := ContextRecord{Title: title, Text: body, Source: source, Time: ts, Truncated: cut}
	if st.cur == nil {
		if len(st.pendingContexts) == 0 {
			st.pendingContextStart = st.off
		}
		st.pendingContexts = append(st.pendingContexts, value)
	} else {
		st.cur.Segments = append(st.cur.Segments, Segment{Context: &value})
	}
}

func (st *parseState) attachContexts() {
	if st.cur == nil || len(st.pendingContexts) == 0 {
		return
	}
	st.curStart = st.pendingContextStart
	for _, value := range st.pendingContexts {
		copy := value
		copy.BeforePrompt = true
		st.cur.Segments = append(st.cur.Segments, Segment{Context: &copy})
	}
	st.pendingContexts = nil
}

// Only complete harness envelopes beginning on their own line outside fenced
// examples are records. Inline mentions and quoted/code examples stay ordinary text.
func (st *parseState) promptContexts(text, source, ts string) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "# AGENTS.md instructions") {
		st.addContext("Workspace instructions", text, source, ts)
		return
	}
	if strings.HasPrefix(trimmed, "<environment_context>") {
		st.addContext("Environment", text, source, ts)
		return
	}
	fence := ""
	for pos := 0; pos < len(text); {
		end := strings.IndexByte(text[pos:], '\n')
		if end < 0 {
			end = len(text) - pos
		}
		line := strings.TrimSpace(text[pos : pos+end])
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			marker := line[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
		} else if fence == "" {
			for _, tag := range []string{"system-reminder", "task-notification"} {
				if !strings.HasPrefix(line, "<"+tag+">") && !strings.HasPrefix(line, "<"+tag+" ") {
					continue
				}
				start := pos + strings.Index(text[pos:pos+end], "<")
				close := "</" + tag + ">"
				offset := strings.Index(text[start:], close)
				if offset >= 0 {
					title := "Environment reminder"
					if tag == "task-notification" {
						title = "Background task notification"
					}
					st.addContext(title, text[start:start+offset+len(close)], source, ts)
					pos = start + offset + len(close)
					end = -1
				}
				break
			}
		}
		pos += end + 1
	}
}
