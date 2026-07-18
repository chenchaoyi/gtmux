package hqwake

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// The outcome tally feeds the summary tick's zero-change gate: it accumulates
// ONLY the outcome-level changes that did NOT already produce an immediate wake
// (attended completions, sessions gone). Hooks append; the serve slow-tick
// consumes. One line per outcome (O_APPEND — atomic for small writes across
// concurrent hook processes).

// tallyPath is the append-only outcome journal for the pending tick.
func tallyPath() string { return filepath.Join(dir(), "tally.log") }

// tickTimePath stamps the last DELIVERED tick (mtime).
func tickTimePath() string { return filepath.Join(dir(), "tick-time") }

// tickSeqPath records the event sequence the last delivered tick covered up to.
func tickSeqPath() string { return filepath.Join(dir(), "tick-seq") }

// AddOutcome appends one outcome ("done" | "gone" | …) to the pending tally.
// Best-effort — perception bookkeeping must never fail a hook.
func AddOutcome(kind string) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return
	}
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(tallyPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(kind + "\n")
	_ = f.Close()
}

// TallyCount returns the number of pending outcomes (cheap line count).
func TallyCount() int {
	f, err := os.Open(tallyPath())
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			n++
		}
	}
	return n
}

// TickDue reports whether the summary tick should fire now: at least one pending
// outcome AND (the minimum interval elapsed since the last delivered tick OR the
// burst threshold is reached). Zero outcomes → never due (the zero-change gate).
func TickDue(now int64, cfg Config) bool {
	n := TallyCount()
	if n == 0 {
		return false
	}
	if n >= cfg.TickBurst {
		return true
	}
	fi, err := os.Stat(tickTimePath())
	if err != nil {
		return true // never delivered → due on the first outcome
	}
	return now-fi.ModTime().Unix() >= cfg.TickMinutes*60
}

// ConsumeTick consumes the pending tally and returns the tick wake line, updating
// the delivery stamps. latestSeq is the current end of the event stream (the tick
// covers (lastCoveredSeq, latestSeq]). Returns "" when the tally emptied between
// the due-check and consumption (harmless race — nothing to say).
func ConsumeTick(now, latestSeq int64) string {
	counts := consumeTally()
	if len(counts) == 0 {
		return ""
	}
	fromSeq := readSeqMarker()
	_ = state.WriteInt64Marker(tickSeqPath(), latestSeq)
	_ = os.WriteFile(tickTimePath(), nil, 0o644)
	t := time.Unix(now, 0)
	_ = os.Chtimes(tickTimePath(), t, t)

	head := fmt.Sprintf("seq %d-%d", fromSeq+1, latestSeq)
	if latestSeq <= fromSeq { // seq counter unavailable → still say something honest
		head = "summary due"
	}
	return Line(ClassTick, head, countsField(counts), PullHint(now, fromSeq))
}

// consumeTally reads + truncates the tally, returning counts by kind.
func consumeTally() map[string]int {
	b, err := os.ReadFile(tallyPath())
	if err != nil {
		return nil
	}
	_ = os.Remove(tallyPath()) // consumed; a concurrent append recreates it for the NEXT tick
	counts := map[string]int{}
	for _, ln := range strings.Split(string(b), "\n") {
		if k := strings.TrimSpace(ln); k != "" {
			counts[k]++
		}
	}
	return counts
}

// countsField renders `2 done · 1 gone` with a stable (sorted) kind order.
func countsField(counts map[string]int) string {
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	return strings.Join(parts, " · ")
}

func readSeqMarker() int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(state.ReadMarker(tickSeqPath())), 10, 64)
	return n
}
