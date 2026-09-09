package usage

// What a model's context window actually is, learned from what this machine has SEEN.
//
// The window is the denominator of every ctx figure gtmux prints, and gtmux cannot read
// it: Claude's log records tokens per message and never the window, and `stats-cache.json`
// carries `contextWindow: 0`. So `windowFor` guessed — smallest known tier at or above the
// tokens observed so far — and that guess is wrong in one direction, systematically:
//
//	a session on a 1M model reads against 200k until it grows past 200k.
//
// Measured on the machine this was written for: HQ was reported at ctx 83% while holding
// ~167k tokens of a model whose own log shows 999,833 in a single request. The real figure
// was 17%. HQ was told to rotate 13 times in one day, each rotation costing it a turn and a
// handoff, and the fresh session was back "over the line" within the hour because the line
// was drawn at a fifth of the real window.
//
// It also explains an observation nobody could account for: ctx reported 98%, then 21% a
// moment later, and it was read as "the harness compacted the context". It was the tier
// flipping. 196k/200k is 98%; cross 200k and the same tokens are judged against 1M.
//
// A model→window TABLE would fix today and rot: names change, the same name ships with
// different windows, and a stale row is a wrong denominator again with no way to notice.
// So this remembers EVIDENCE instead — the largest context a model has ever been seen to
// hold on this machine. A window is never smaller than something that fit in it, so the
// high-water mark is a sound floor, it needs no maintenance, and it self-corrects the
// moment a bigger context appears.
//
// The cost is honest and bounded: the very first session of a new model still reads
// against the tier its current size falls in, until it grows. It heals itself.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// windowSeenPath holds the per-model high-water marks.
func windowSeenPath() string { return filepath.Join(state.Dir(), "model-windows.json") }

var windowSeenMu sync.Mutex

// modelKey is what the high-water is filed under. Agent-qualified, because two agents can
// use the same model name against different deployments.
func modelKey(agent, model string) string {
	a := strings.ToLower(strings.TrimSpace(agent))
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return ""
	}
	return a + "/" + m
}

func loadWindowSeen() map[string]int64 {
	b, err := os.ReadFile(windowSeenPath())
	if err != nil {
		return map[string]int64{}
	}
	var m map[string]int64
	if json.Unmarshal(b, &m) != nil || m == nil {
		return map[string]int64{}
	}
	return m
}

// noteWindowEvidence records that `tokens` fit in this model's window.
//
// Written only when it RAISES the mark, so the common poll is a read. Best-effort
// throughout: this is an optimisation of a denominator, never a reason to fail a report.
func noteWindowEvidence(agent, model string, tokens int64) {
	key := modelKey(agent, model)
	if key == "" || tokens <= 0 {
		return
	}
	windowSeenMu.Lock()
	defer windowSeenMu.Unlock()
	seen := loadWindowSeen()
	if seen[key] >= tokens {
		return
	}
	seen[key] = tokens
	if os.MkdirAll(state.Dir(), 0o755) != nil {
		return
	}
	if b, err := json.Marshal(seen); err == nil {
		_ = os.WriteFile(windowSeenPath(), b, 0o644)
	}
}

// evidencedWindow is the largest context this model has been seen to hold, or 0.
func evidencedWindow(agent, model string) int64 {
	key := modelKey(agent, model)
	if key == "" {
		return 0
	}
	windowSeenMu.Lock()
	defer windowSeenMu.Unlock()
	return loadWindowSeen()[key]
}

// ── one-time backfill ────────────────────────────────────────────────────────
//
// Evidence gathered only from LIVE sessions has a hole it cannot climb out of, and HQ
// fell in it: its model is judged against 200k, so at 150k it is told to rotate — which
// means no session of that model ever reaches the size that would prove the window is
// bigger. The false alarm prevents the evidence that would silence it.
//
// The history is right there. Every past session's log records its model and its token
// counts, and on this machine those logs show the very model HQ runs having held 998,881
// tokens. One pass over them, once, turns a self-defeating heuristic into a correct one.

// backfillMarker records that the history has been read, so this happens once.
func backfillMarker() string { return filepath.Join(state.Dir(), "model-windows.scanned") }

// BackfillWindows scans the agent logs already on disk for each model's largest observed
// context, once. Called from the resident tick — a periodic job with no resident trigger
// does not run at all, which this codebase has paid for before.
//
// Cheap by construction: only the TAIL of each log is read. A session's largest context is
// its last full request, so the end of the file is where the answer is, and reading the
// whole of 767 logs to find it would be work nobody asked for.
func BackfillWindows() {
	if _, err := os.Stat(backfillMarker()); err == nil {
		return
	}
	for agent, glob := range map[string]string{
		"claude": filepath.Join(state.Home(), ".claude", "projects", "*", "*.jsonl"),
	} {
		paths, _ := filepath.Glob(glob)
		for _, p := range paths {
			model, ctx := tailModelCtx(p)
			if model != "" && ctx > 0 {
				noteWindowEvidence(agent, model, ctx)
			}
		}
	}
	if os.MkdirAll(state.Dir(), 0o755) == nil {
		_ = os.WriteFile(backfillMarker(), []byte("1"), 0o644)
	}
}

// backfillTail is how much of each log's end is read. Large enough to hold the closing
// exchange of any ordinary session, small enough that a whole history is a bounded read.
const backfillTail = 256 << 10

// tailModelCtx reads a log's tail and returns the model and the largest context in it.
func tailModelCtx(path string) (string, int64) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", 0
	}
	start := int64(0)
	if fi.Size() > backfillTail {
		start = fi.Size() - backfillTail
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil {
		return "", 0
	}
	var model string
	var best int64
	for _, line := range strings.Split(string(buf), "\n") {
		m, ok := parseLine([]byte(line))
		if !ok {
			continue
		}
		if m.model != "" {
			model = m.model
		}
		if c := m.ctxTokens(); c > best {
			best = c
		}
	}
	return model, best
}
