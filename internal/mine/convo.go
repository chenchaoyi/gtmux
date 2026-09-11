package mine

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// convo is the one conversation state machine every agent's reader feeds. An agent's
// log differs in envelope (Claude's records, Codex's payloads, Kimi's journal, the
// transcript gtmux writes for opencode) but not in what the miner needs from it: who
// spoke, what tool ran, what it printed. Each reader translates its envelope into these
// four calls; the judgement (machine subtraction, paste bound, lexicon, error
// signature) lives here once.
type convo struct {
	o   readOpts
	res readResult

	lastAssistant string
	spoke         bool              // the assistant has spoken since the last human line
	tools         map[string]string // tool-call id → name, results pending
	shell         func(name string) bool
	session       string
	project       string
}

func newConvo(o readOpts, shell func(string) bool) *convo {
	c := &convo{o: o, res: readResult{errors: map[string]errorHit{}}, shell: shell, tools: map[string]string{}}
	c.lastAssistant, c.spoke = o.carry.Tail, o.carry.Spoke
	for id, name := range o.carry.Tools {
		c.tools[id] = name
	}
	return c
}

func (c *convo) meta(session, cwd string) {
	if session != "" {
		c.session = session
	}
	if cwd != "" {
		c.project = filepath.Base(cwd)
	}
}

func (c *convo) assistant(text string) {
	if t := strings.TrimSpace(text); t != "" {
		c.lastAssistant = t
		c.spoke = true
	}
}

func (c *convo) toolCall(id, name string) {
	if id != "" {
		c.tools[id] = name
	}
}

// toolResult tallies an error signature when the tool was a shell and its output
// holds a hard failure.
func (c *convo) toolResult(id, output string, at int64) {
	name := c.tools[id]
	delete(c.tools, id)
	if !c.shell(name) {
		return
	}
	sig, ok := errorSignature(output)
	if !ok {
		return
	}
	h := c.res.errors[sig]
	h.line, h.session = sig, c.session
	h.count++
	h.at = maxInt64(h.at, at)
	c.res.errors[sig] = h
}

// human is a line the agent's log attributes to the user. The subtractions that are
// agent-independent run here: harness blocks and gtmux's wake lines (the hook's own
// classifier), `gtmux send` heads, pastes. Envelope-specific drops (a Codex
// `<environment_context>` block, a Kimi injection origin) happen in the reader before
// this call — a reader passes only what its agent would have shown as a typed turn.
func (c *convo) human(raw string, at int64, uid string) {
	text, kind := transcript.ClassifyUserPrompt(raw)
	if kind != transcript.PromptUser {
		return
	}
	spoke := c.spoke
	c.spoke = false
	if c.o.machine[HeadKey(text)] {
		return // the machine typed this (gtmux send) — not the human
	}
	if utf8.RuneCountInString(text) > pasteRunes {
		return
	}
	c.res.humanLines++
	if at < c.o.sinceUnix || !spoke || !isCorrectionShaped(text) {
		return
	}
	c.res.corrections = append(c.res.corrections, Candidate{
		Kind: KindCorrection, ID: candidateID(c.session, uid), At: at,
		Session: c.session, Project: c.project,
		Line:    clipRunes(strings.Join(strings.Fields(text), " "), lineRunes),
		Context: tailRunes(strings.Join(strings.Fields(c.lastAssistant), " "), contextRunes),
	})
}

func (c *convo) finish() readResult {
	c.res.carry = carryState{Spoke: c.spoke, Tail: tailRunes(c.lastAssistant, contextRunes), Tools: pendingTools(c.tools)}
	return c.res
}

// readLog walks a log from start to its last COMPLETE line, handing each line to step,
// and returns the offset after that line — a trailing partial line (the agent
// mid-write) is left for the next pass.
func readLog(path string, start int64, o readOpts, shell func(string) bool, step func(line string, c *convo)) (res readResult, end int64, err error) {
	c := newConvo(o, shell)
	f, err := os.Open(path)
	if err != nil {
		return c.finish(), start, err
	}
	defer f.Close()
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return c.finish(), start, err
	}
	r := bufio.NewReaderSize(f, 256*1024)
	off := start
	for {
		line, rerr := r.ReadString('\n')
		if rerr != nil && rerr != io.EOF {
			return c.finish(), off, rerr
		}
		if rerr == io.EOF && !strings.HasSuffix(line, "\n") {
			break // partial tail — not consumed
		}
		if len(line) == 0 {
			break
		}
		off += int64(len(line))
		step(line, c)
		if rerr == io.EOF {
			break
		}
	}
	return c.finish(), off, nil
}

// pendingTools keeps the tool-call ids whose result has not arrived, bounded — a
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

// Bounds. A typed line longer than pasteRunes is a paste, not a reaction; the clips keep
// a spool line readable.
const (
	pasteRunes   = 1500
	lineRunes    = 300
	contextRunes = 240
)

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

type readOpts struct {
	sinceUnix int64
	machine   map[string]bool
	// carry is the conversational state at the offset a pass resumes from: whether the
	// assistant had spoken since the last human line, the tail of what it said, and the
	// tool-call ids still waiting for a result. Without it a pass that resumes right
	// after an assistant reply cannot see that the next human line answers one.
	carry carryState
}

// carryState is persisted per file in the ledger.
type carryState struct {
	Spoke bool              `json:"spoke,omitempty"`
	Tail  string            `json:"tail,omitempty"`
	Tools map[string]string `json:"tools,omitempty"` // tool-call id → name, results pending
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
