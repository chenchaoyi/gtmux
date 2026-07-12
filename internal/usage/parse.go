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
type msg struct {
	at          time.Time
	model       string
	in          int64 // non-cached input tokens
	out         int64
	cacheRead   int64
	cacheCreate int64
}

// logLine is the minimal decode of a session-log line (Claude shape; Codex has
// no per-message usage yet — its lines simply won't match and yield nothing).
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

// parseLine extracts a usage message from one raw log line, ok=false when the
// line carries no assistant usage.
func parseLine(line []byte) (msg, bool) {
	// Cheap pre-filter: the vast majority of lines have no usage block.
	if !strings.Contains(string(line), `"usage"`) {
		return msg{}, false
	}
	var l logLine
	if json.Unmarshal(line, &l) != nil || l.Message.Role != "assistant" {
		return msg{}, false
	}
	u := l.Message.Usage
	if u.In == 0 && u.Out == 0 && u.CacheRead == 0 && u.CacheCreate == 0 {
		return msg{}, false
	}
	at, err := time.Parse(time.RFC3339, l.Timestamp)
	if err != nil {
		at = time.Time{}
	}
	return msg{at: at, model: l.Message.Model, in: u.In, out: u.Out,
		cacheRead: u.CacheRead, cacheCreate: u.CacheCreate}, true
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

// windowFor is the model's context window for the ctx fraction. A configured
// per-agent-type override (usage.json `window`) wins. Otherwise the window is
// INFERRED from evidence: a session's observed context cannot exceed its real
// window, so pick the smallest known tier ≥ the observed footprint (model-name
// strings don't reliably encode long-context variants — dogfood showed
// "claude-fable-5"/"claude-opus-4-8" both running 1M sessions).
func windowFor(agent, model string, observed int64) int64 {
	_ = model // kept for a future name-keyed table; evidence wins today
	if w := configWindow(agent); w > 0 {
		return w
	}
	for _, t := range windowTiers {
		if observed <= t {
			return t
		}
	}
	return windowTiers[len(windowTiers)-1]
}
