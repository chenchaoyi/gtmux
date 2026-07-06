// Package transcript turns an agent's on-disk conversation log into a compact
// list of Turns — {prompt, final response, collapsed middle steps} — for the
// mobile/web "对话 (chat)" view. Each agent writes its log in its OWN format, so
// there is one parser per agent (claude.go, codex.go) over a SHARED output model.
//
// The pane→session mapping lives elsewhere (resume.Record gives agent+sessionId);
// here we only take that record and read the log. Files are looked up by GLOB on
// the session id, so we never have to reconstruct an agent's cwd-encoding scheme.
package transcript

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// tsField matches a log line's "timestamp":"<RFC3339>" — the wall-clock the agent
// stamped that message with.
var tsField = regexp.MustCompile(`"timestamp":"([^"]+)"`)

// LastMessageTime returns the wall-clock (unix seconds) of the LAST message logged
// in an agent's session — when it most recently did anything, i.e. ~when its turn
// ended. It reads the log's TAIL and parses the last "timestamp" field, so it
// reflects real activity — unlike the file mtime, which a resume/rewrite bumps
// without adding messages. 0 when the log or a timestamp can't be found.
func LastMessageTime(agent, sessionID string) int64 {
	path, _ := resolveLog(agent, sessionID)
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0
	}
	const tail = 64 << 10
	start := int64(0)
	if fi.Size() > tail {
		start = fi.Size() - tail
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return 0
	}
	m := tsField.FindAllStringSubmatch(string(buf), -1)
	if len(m) == 0 {
		return 0
	}
	ts := m[len(m)-1][1]
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Unix()
	}
	return 0
}

// LastMessageError reports whether an agent's session ENDED on an API/tool error —
// i.e. the LAST real message in its Claude Code transcript is an entry flagged
// `isApiErrorMessage:true` (e.g. "API Error: Unable to connect to API"). It returns
// a short summary for display. Mid-turn retry errors that recovered are NOT flagged
// (their last message is a normal assistant/user entry). Non-Claude or unreadable
// logs → (false, ""). Reads the same tail as LastMessageTime.
func LastMessageError(agent, sessionID string) (bool, string) {
	path, _ := resolveLog(agent, sessionID)
	if path == "" {
		return false, ""
	}
	f, err := os.Open(path)
	if err != nil {
		return false, ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, ""
	}
	const tail = 64 << 10
	start := int64(0)
	if fi.Size() > tail {
		start = fi.Size() - tail
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return false, ""
	}
	lines := strings.Split(string(buf), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:] // drop the partial first line (tail began mid-line)
	}
	// Scan backward to the last REAL message (skip blanks, meta, sidechain, and
	// non-message events). That message decides how the session ended.
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			continue
		}
		var e claudeLine
		if json.Unmarshal([]byte(s), &e) != nil || e.IsMeta || e.IsSidechain || e.Message == nil {
			continue
		}
		if e.IsAPIError {
			return true, errorSummary(e.Message.Content)
		}
		return false, "" // the last real message was normal → completed
	}
	return false, ""
}

// errorSummary pulls a short, one-line error string out of a Claude error entry's
// content (a bare string, or a text block), trimmed of the "API Error:" prefix and
// truncated for a radar row.
func errorSummary(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) != nil {
		var blocks []claudeBlock
		if json.Unmarshal(content, &blocks) == nil {
			for _, b := range blocks {
				if b.Text != "" {
					s = b.Text
					break
				}
			}
		}
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(s, "API Error:"))
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	const max = 80
	if len(s) > max {
		s = strings.TrimSpace(s[:max]) + "…"
	}
	return s
}

// Step is one collapsed middle action in a turn (a tool call). The chat view
// shows turns as prompt → (N steps, collapsed) → final response.
type Step struct {
	Kind   string `json:"kind"`             // "tool" for now
	Title  string `json:"title"`            // tool name (Edit, Bash, exec_command…)
	Detail string `json:"detail,omitempty"` // a short arg summary (path / command head)
}

// Turn is one user instruction and what the agent ultimately answered, with the
// intermediate tool calls folded into Steps.
type Turn struct {
	Prompt string `json:"prompt"`
	// Response is the full reply (all segment texts joined by a blank line) — kept
	// for back-compat and simple consumers.
	Response string `json:"response"`
	// Segments are the reply in chronological order: an agent emits a turn as
	// several assistant messages interleaved with tool calls (text → tools → text →
	// …). Each Segment is one text bubble plus the tool steps that ran AFTER it
	// (until the next text), so the chat can render each text as its own speech
	// bubble with the intermediate process shown between bubbles.
	Segments []Segment `json:"segments,omitempty"`
	// Time is the prompt's wall-clock timestamp (RFC3339, as logged by the agent),
	// for the chat view's per-turn time label. "" when the log line carried none.
	Time string `json:"time,omitempty"`
}

// Segment is one chronological piece of a turn's reply: an assistant text bubble
// and the tool calls that followed it. Either side may be empty (a tools-only
// segment renders as just the collapsed step group; a text-only one as a bubble).
type Segment struct {
	Text  string `json:"text,omitempty"`
	Steps []Step `json:"steps,omitempty"`
}

// maxTailBytes bounds how much of a (potentially huge) log we read: only the tail
// matters for "recent history", and a partial first turn is simply dropped. 8 MiB
// covers a lot of turns (so the chat view can show deep history); a single
// hyperactive in-progress turn can still exceed it, in which case it surfaces as
// one prompt-less "current activity" card.
const maxTailBytes = 64 << 20 // 64 MiB

// Load returns up to maxTurns most-recent turns (oldest-first) for an agent
// session. It is backed by a process-wide incremental cache (see Loader): the
// first call deep-tails the log, and subsequent polls of a GROWING log re-parse
// only from the last turn's start — so the chat endpoint can poll a 100 MiB+
// agentic log every couple of seconds without re-reading it each time. Unknown
// agent or missing log → (nil, nil): the caller renders "no history yet".
func Load(agent, sessionID string, maxTurns int) ([]Turn, error) {
	return defaultLoader.load(agent, sessionID, maxTurns)
}

// stepFn folds one decoded log line into the running parse state. Each agent has
// its own (claudeStep / codexStep) over the SHARED parseState.
type stepFn func(line string, st *parseState)

// resolveLog maps an agent + session id to its on-disk log path and parser.
func resolveLog(agent, sessionID string) (string, stepFn) {
	switch normalizeAgent(agent) {
	case "claude":
		return claudeLogPath(sessionID), claudeStep
	case "codex":
		return codexLogPath(sessionID), codexStep
	}
	return "", nil
}

// parseState accumulates turns as a parser walks the log linearly, tracking the
// byte offset at which each turn began so the cache can restart incrementally.
type parseState struct {
	turns      []Turn
	turnStarts []int64 // turnStarts[i] = byte offset of turns[i]'s first line
	cur        *Turn
	curStart   int64
	off        int64 // start offset of the line currently being fed to a step
}

// flush commits the in-progress turn (if it carries any content) and records
// where it started. Response is derived here from the segment texts.
func (st *parseState) flush() {
	if st.cur != nil && (st.cur.Prompt != "" || len(st.cur.Segments) > 0) {
		var texts []string
		for _, s := range st.cur.Segments {
			if s.Text != "" {
				texts = append(texts, s.Text)
			}
		}
		st.cur.Response = strings.Join(texts, "\n\n")
		st.turns = append(st.turns, *st.cur)
		st.turnStarts = append(st.turnStarts, st.curStart)
	}
	st.cur = nil
}

// addText starts a new reply segment (a text bubble).
func (st *parseState) addText(text string) {
	st.ensure()
	st.cur.Segments = append(st.cur.Segments, Segment{Text: text})
}

// lastSegmentText reports whether the current turn's last segment text already
// equals s — so a Codex task_complete doesn't duplicate the closing agent_message.
func lastSegmentText(st *parseState, s string) bool {
	if st.cur == nil || len(st.cur.Segments) == 0 {
		return false
	}
	return st.cur.Segments[len(st.cur.Segments)-1].Text == s
}

// addSteps attaches tool steps to the current segment (the tools that ran after the
// last text bubble); if none exists yet, opens a leading tools-only segment so the
// steps that preceded the first text still appear in order.
func (st *parseState) addSteps(steps []Step) {
	if len(steps) == 0 {
		return
	}
	st.ensure()
	if len(st.cur.Segments) == 0 {
		st.cur.Segments = append(st.cur.Segments, Segment{})
	}
	last := &st.cur.Segments[len(st.cur.Segments)-1]
	last.Steps = append(last.Steps, steps...)
}

// open starts a fresh turn at a user prompt, closing the previous one. ts is the
// prompt's log timestamp (RFC3339), used for the chat view's time label.
func (st *parseState) open(prompt, ts string) {
	st.flush()
	st.cur = &Turn{Prompt: prompt, Time: ts}
	st.curStart = st.off
}

// ensure opens a prompt-less turn for assistant output that arrived with no
// preceding prompt (the tail began mid-turn).
func (st *parseState) ensure() {
	if st.cur == nil {
		st.cur = &Turn{}
		st.curStart = st.off
	}
}

// parseTurns streams path from startOffset through step. Pass startOffset = -1 to
// "tail" the last maxTailBytes (dropping the partial first line); pass a known
// line-aligned offset (from a prior parse) to resume incrementally. Returns the
// turns, the byte offset where the LAST turn began (-1 if none), and the file
// size it parsed to.
func parseTurns(path string, startOffset int64, step stepFn) ([]Turn, int64, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, -1, 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, -1, 0, err
	}
	size := fi.Size()

	start := startOffset
	dropPartial := false
	if start < 0 { // tail mode
		start = 0
		if size > maxTailBytes {
			start = size - maxTailBytes
			dropPartial = true // we seeked into the middle of a line
		}
	}
	if start > size {
		start = size
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, -1, size, err
	}

	r := bufio.NewReaderSize(f, 64*1024)
	off := start
	if dropPartial {
		first, _ := r.ReadString('\n')
		off += int64(len(first))
	}
	st := &parseState{off: off}
	for {
		line, rerr := r.ReadString('\n')
		if len(line) > 0 {
			st.off = off
			if strings.TrimSpace(line) != "" {
				step(strings.TrimRight(line, "\n"), st)
			}
			off += int64(len(line))
		}
		if rerr != nil {
			break
		}
	}
	st.flush()
	last := int64(-1)
	if n := len(st.turnStarts); n > 0 {
		last = st.turnStarts[n-1]
	}
	return st.turns, last, size, nil
}

// cacheTurnCap bounds how many turns the cache retains per session, so a
// long-lived session can't grow the cache without limit. Trimming the FRONT is
// safe — we only ever extend forward, and the incremental restart depends on the
// last turn's offset, not the front.
const cacheTurnCap = 800

type cacheEntry struct {
	path     string
	size     int64
	turns    []Turn
	lastTurn int64 // byte offset of the last turn's start (-1 if none)
}

// Loader caches parsed transcripts per session and re-parses incrementally as the
// log grows (see Load).
type Loader struct {
	mu sync.Mutex
	m  map[string]*cacheEntry
}

var defaultLoader = &Loader{m: map[string]*cacheEntry{}}

func (l *Loader) load(agent, sessionID string, maxTurns int) ([]Turn, error) {
	if sessionID == "" {
		return nil, nil
	}
	path, step := resolveLog(agent, sessionID)
	if path == "" || step == nil {
		return nil, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if hit := l.m[sessionID]; hit != nil && hit.path == path {
		if fi, err := os.Stat(path); err == nil {
			switch {
			case fi.Size() == hit.size: // unchanged → serve cache
				return lastN(hit.turns, maxTurns), nil
			case fi.Size() > hit.size && hit.lastTurn >= 0 && len(hit.turns) > 0:
				// grew → re-parse only from the last turn's start and splice.
				tail, last, size, perr := parseTurns(path, hit.lastTurn, step)
				if perr == nil {
					merged := make([]Turn, 0, len(hit.turns)-1+len(tail))
					merged = append(merged, hit.turns[:len(hit.turns)-1]...)
					merged = append(merged, tail...)
					newLast := hit.lastTurn
					if len(tail) > 0 {
						newLast = last
					}
					if len(merged) > cacheTurnCap {
						merged = append([]Turn(nil), merged[len(merged)-cacheTurnCap:]...)
					}
					l.m[sessionID] = &cacheEntry{path: path, size: size, turns: merged, lastTurn: newLast}
					return lastN(merged, maxTurns), nil
				}
			}
		}
	}

	// cold / shrunk / rotated → full deep-tail parse.
	turns, last, size, err := parseTurns(path, -1, step)
	if err != nil {
		return nil, err
	}
	if len(turns) > cacheTurnCap {
		turns = append([]Turn(nil), turns[len(turns)-cacheTurnCap:]...)
	}
	l.m[sessionID] = &cacheEntry{path: path, size: size, turns: turns, lastTurn: last}
	if len(turns) == 0 {
		return nil, nil
	}
	return lastN(turns, maxTurns), nil
}

// normalizeAgent maps a display name or key ("Claude Code", "claude", "Codex")
// to a parser key.
func normalizeAgent(agent string) string {
	a := strings.ToLower(agent)
	switch {
	case strings.Contains(a, "claude"):
		return "claude"
	case strings.Contains(a, "codex"):
		return "codex"
	}
	return a
}

// lastN trims turns to the most recent n (oldest-first order preserved).
func lastN(turns []Turn, n int) []Turn {
	if n > 0 && len(turns) > n {
		return turns[len(turns)-n:]
	}
	return turns
}

// clip shortens s to max runes with an ellipsis, collapsing whitespace — for a
// step's one-line detail.
func clip(s string, max int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
