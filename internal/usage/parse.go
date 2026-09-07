package usage

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"
)

// msg is one usage-bearing log line's extract.
//
// Agents log usage in two shapes and the difference is not cosmetic: Claude
// writes what THIS message cost, so the totals are a sum; Codex writes the
// session's RUNNING TOTALS on every turn, so the totals are the last reading.
// Summing a running total would multiply a session's burn by its turn count, so
// `cumulative` says which half of this struct means anything.
type msg struct {
	at    time.Time
	model string

	// Per-message deltas (cumulative == false).
	in          int64 // non-cached input tokens
	out         int64
	cacheRead   int64
	cacheCreate int64

	// A running-total observation (cumulative == true).
	cumulative bool
	totalIn    int64 // cumulative non-cached input, as of this line
	totalOut   int64
	statedCtx  int64 // live context footprint the log states outright
	window     int64 // context window the log states outright (0 = infer it)
}

// ctxTokens is the live context footprint this observation implies.
func (m msg) ctxTokens() int64 {
	if m.cumulative {
		return m.statedCtx
	}
	return m.in + m.cacheRead + m.cacheCreate
}

// logLine is the minimal decode of a Claude session-log line: usage per
// assistant message, as a delta.
type logLine struct {
	Timestamp string `json:"timestamp"`
	Message   struct {
		Role  string `json:"role"`
		Model string `json:"model"`
		Usage struct {
			In          int64 `json:"input_tokens"`
			Out         int64 `json:"output_tokens"`
			CacheRead   int64 `json:"cache_read_input_tokens"`
			CacheCreate int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// codexTokens is one side of a Codex `token_count` payload. `input_tokens` is
// the WHOLE input including the cached part, which `cached_input_tokens`
// re-states — so the non-cached figure comparable to Claude's is their
// difference, and the context footprint is `input_tokens` as it stands.
type codexTokens struct {
	In     int64 `json:"input_tokens"`
	Cached int64 `json:"cached_input_tokens"`
	Out    int64 `json:"output_tokens"`
}

// codexLine is the minimal decode of a Codex rollout line. Codex emits a
// `token_count` event after each turn carrying the session's totals, the last
// turn's usage, and — unlike Claude — the model's context window outright.
type codexLine struct {
	Timestamp string `json:"timestamp"`
	Payload   struct {
		Type string `json:"type"`
		Info struct {
			Total  codexTokens `json:"total_token_usage"`
			Last   codexTokens `json:"last_token_usage"`
			Window int64       `json:"model_context_window"`
		} `json:"info"`
	} `json:"payload"`
}

// parseLine extracts a usage observation from one raw log line, ok=false when
// the line carries none.
func parseLine(line []byte) (msg, bool) {
	// Cheap pre-filter: the vast majority of lines carry no usage at all. Both
	// markers are needed — Codex's block is `"total_token_usage"`, which does
	// NOT contain the substring `"usage"` (the quote sits before `total`).
	s := string(line)
	if !strings.Contains(s, `"usage"`) && !strings.Contains(s, `"token_count"`) {
		return msg{}, false
	}
	if m, ok := parseCodexLine(line); ok {
		return m, true
	}
	var l logLine
	if json.Unmarshal(line, &l) != nil || l.Message.Role != "assistant" {
		return msg{}, false
	}
	u := l.Message.Usage
	if u.In == 0 && u.Out == 0 && u.CacheRead == 0 && u.CacheCreate == 0 {
		return msg{}, false
	}
	return msg{at: parseAt(l.Timestamp), model: l.Message.Model, in: u.In, out: u.Out,
		cacheRead: u.CacheRead, cacheCreate: u.CacheCreate}, true
}

// parseCodexLine decodes a Codex `token_count` event.
func parseCodexLine(line []byte) (msg, bool) {
	var c codexLine
	if json.Unmarshal(line, &c) != nil || c.Payload.Type != "token_count" {
		return msg{}, false
	}
	t, last := c.Payload.Info.Total, c.Payload.Info.Last
	if t.In == 0 && t.Out == 0 {
		return msg{}, false
	}
	return msg{
		at:         parseAt(c.Timestamp),
		cumulative: true,
		totalIn:    nonNegative(t.In - t.Cached),
		totalOut:   t.Out,
		statedCtx:  last.In,
		window:     c.Payload.Info.Window,
	}, true
}

// parseAt reads a log timestamp, zero when it is missing or malformed — never a
// guessed one, since an invented time would land in the rate window.
func parseAt(s string) time.Time {
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return at
}

func nonNegative(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// scanFrom folds usage messages from byte offset `from` to `to`, returning the
// new offset (== to) and the messages found. Line-oriented; a partial trailing
// line is left for the next scan by stopping at the last newline.
func scanFrom(path string, from, to int64) (int64, []msg) {
	f, err := os.Open(path)
	if err != nil {
		return from, nil
	}
	defer f.Close()
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return from, nil
	}
	r := bufio.NewReaderSize(io.LimitReader(f, to-from), 1<<20)
	var out []msg
	consumed := from
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// no trailing newline → leave the partial line for next time
			break
		}
		consumed += int64(len(line))
		if m, ok := parseLine(line); ok {
			out = append(out, m)
		}
	}
	return consumed, out
}

// tailMessages parses the last tailBytes of the log for the context footprint +
// the rate window. The first (possibly partial) line is skipped.
func tailMessages(path string, size int64) []msg {
	from := size - tailBytes
	if from < 0 {
		from = 0
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return nil
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	first := from != 0
	var out []msg
	for sc.Scan() {
		if first { // mid-line entry point — drop the fragment
			first = false
			continue
		}
		if m, ok := parseLine(sc.Bytes()); ok {
			out = append(out, m)
		}
	}
	return out
}

// windowTiers are the known context-window sizes, ascending.
var windowTiers = []int64{200_000, 1_000_000}

// windowFor is the model's context window for the ctx fraction, best source
// first:
//
//  1. a configured per-agent-type override (usage.json `window`) — you said so;
//  2. `stated`, when the log says it outright (Codex reports
//     `model_context_window` on every turn) — the agent knows, so do not guess;
//  3. otherwise INFER from evidence: a session's observed context cannot exceed
//     its real window, so pick the smallest known tier ≥ the observed footprint.
//     Model-name strings don't reliably encode long-context variants — dogfood
//     showed "claude-fable-5"/"claude-opus-4-8" both running 1M sessions.
func windowFor(agent, model string, observed, stated int64) int64 {
	if w := configWindow(agent); w > 0 {
		return w
	}
	if stated > 0 {
		return stated
	}
	// The largest context this model has EVER been seen to hold on this machine. A
	// window is never smaller than something that fit in it, so this is a sound floor —
	// and it is the whole fix for the tier guess below, which reads a session on a 1M
	// model against 200k until it happens to grow past 200k. See modelwindow.go for the
	// measurement that produced this (HQ reported at 83% while actually at 17%, and told
	// to rotate thirteen times in a day because of it).
	if seen := evidencedWindow(agent, model); seen > observed {
		observed = seen
	}
	for _, t := range windowTiers {
		if observed <= t {
			return t
		}
	}
	return windowTiers[len(windowTiers)-1]
}
