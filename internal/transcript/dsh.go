package transcript

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

type dshContent struct{ Type, Text string }
type dshMessage struct {
	Role    string
	Content []dshContent
	Source  struct{ Kind string }
	ID      string
}
type dshCacheEntry struct {
	cursor  dsh.Cursor
	state   parseState
	header  bool
	dropped int
}

// Only user-authored prompts and visible text blocks become chat. Model replay
// state, reasoning blocks, injected instructions and tool results stay out.
func dshStep(line []byte, st *parseState) error {
	var event dsh.Event
	if err := json.Unmarshal(line, &event); err != nil {
		return err
	}
	if !event.AppendsHistory() {
		return nil
	}
	switch event.Type {
	case "user/message":
		var msg dshMessage
		if err := json.Unmarshal(event.Data, &msg); err != nil {
			return err
		}
		if msg.Source.Kind != "user" || msg.Role != "user" {
			return nil
		}
		if text := dshText(msg.Content); text != "" {
			st.open(text, dsh.Timestamp(event.Time))
			seq := event.Seq
			st.cur.PromptMessageSeq = &seq
		}
	case "assistant/message":
		var data struct{ Message dshMessage }
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if data.Message.Role != "assistant" {
			return nil
		}
		if text := dshText(data.Message.Content); text != "" {
			st.addText(text)
		}
	case "tool/call":
		var data struct{ Name, Arguments string }
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if len(data.Name) > 256 {
			return fmt.Errorf("DSH tool name exceeds read limit")
		}
		if data.Name != "" {
			st.addSteps([]Step{toolStep(data.Name, data.Name, json.RawMessage(data.Arguments), toolDetail(json.RawMessage(data.Arguments)))})
		}
	}
	if st.cur != nil {
		size := len(st.cur.Prompt)
		size += st.toolDetailBytes
		for _, seg := range st.cur.Segments {
			size += len(seg.Text) + len(seg.Steps)*200
		}
		if size > 2<<20 || len(st.cur.Segments) > 4096 {
			return fmt.Errorf("DSH turn exceeds public-history read limit")
		}
	}
	return nil
}
func dshText(blocks []dshContent) string {
	var out []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			out = append(out, b.Text)
		}
	}
	return strings.Join(out, "\n")
}

func (l *Loader) loadDSH(id string, maxTurns int) ([]Turn, int, error) {
	session, err := dsh.Sessions.Resolve(id)
	if err != nil {
		return nil, 0, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dsh == nil {
		l.dsh = map[string]*dshCacheEntry{}
	}
	e := l.dsh[session.Path]
	if e == nil {
		if len(l.dsh) >= 16 {
			for k := range l.dsh {
				delete(l.dsh, k)
				break
			}
		}
		e = &dshCacheEntry{}
		l.dsh[session.Path] = e
	}
	err = dsh.Scan(session.Path, &e.cursor, func() { e.state = parseState{}; e.header = false; e.dropped = 0 }, func(line []byte) error {
		if !e.header {
			var h dsh.Header
			if json.Unmarshal(line, &h) != nil || h.Type != "session" || h.Version != 4 || h.ID != id {
				return fmt.Errorf("DSH transcript identity changed")
			}
			e.header = true
			return nil
		}
		if err := dshStep(line, &e.state); err != nil {
			return err
		}
		// Retain the same newest-turn window as other parsers, with a byte cap too.
		for len(e.state.turns) > 0 && (len(e.state.turns) > cacheTurnCap || dshTurnBytes(e.state.turns) > 4<<20) {
			e.state.turns = append([]Turn(nil), e.state.turns[1:]...)
			e.state.turnStarts = append([]int64(nil), e.state.turnStarts[1:]...)
			e.dropped++
		}
		return nil
	})
	if err != nil {
		delete(l.dsh, session.Path)
		return nil, 0, err
	}
	snapshot := e.state
	snapshot.turns = append([]Turn(nil), e.state.turns...)
	if e.state.cur != nil {
		turn := *e.state.cur
		turn.Segments = append([]Segment(nil), turn.Segments...)
		snapshot.cur = &turn
	}
	snapshot.flush()
	turns := lastN(snapshot.turns, maxTurns)
	dropped := e.dropped + len(snapshot.turns) - len(turns)
	// HTTP/driver consumers can retain and annotate their snapshot safely.
	encoded, err := json.Marshal(turns)
	if err != nil {
		return nil, 0, err
	}
	var out []Turn
	err = json.Unmarshal(encoded, &out)
	for i := range out {
		if turns[i].PromptMessageSeq != nil {
			seq := *turns[i].PromptMessageSeq
			out[i].PromptMessageSeq = &seq
		}
	}
	return out, dropped, err
}
func dshTurnBytes(turns []Turn) int {
	n := 0
	for _, t := range turns {
		n += len(t.Prompt) + len(t.Response)
		for _, s := range t.Segments {
			n += len(s.Text) + len(s.Steps)*200
			for _, step := range s.Steps {
				n += len(step.Input) + len(step.Workdir)
			}
		}
	}
	return n
}
func LoadDSH(id string, maxTurns int) ([]Turn, int, error) {
	return defaultLoader.loadDSH(id, maxTurns)
}
