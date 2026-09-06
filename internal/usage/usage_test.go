package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A tiny synthetic session log in the Claude shape.
func writeLog(t *testing.T, dir string, lines []string) string {
	t.Helper()
	p := filepath.Join(dir, "s1.jsonl")
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func asst(ts string, in, out, cr, cc int64) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":"%s","message":{"role":"assistant","model":"claude-x","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`,
		ts, in, out, cr, cc)
}

func TestScanAndTail(t *testing.T) {
	dir := t.TempDir()
	p := writeLog(t, dir, []string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		asst("2026-07-12T10:00:00Z", 5, 100, 1000, 200),
		asst("2026-07-12T10:05:00Z", 2, 300, 1500, 100),
	})
	fi, _ := os.Stat(p)
	off, msgs := scanFrom(p, 0, fi.Size())
	if off != fi.Size() || len(msgs) != 2 {
		t.Fatalf("scanFrom = off %d msgs %d, want %d / 2", off, len(msgs), fi.Size())
	}
	if msgs[1].out != 300 || msgs[1].cacheRead != 1500 {
		t.Errorf("msg parse: %+v", msgs[1])
	}
	tail := tailMessages(p, fi.Size())
	if len(tail) != 2 {
		t.Fatalf("tailMessages = %d, want 2", len(tail))
	}
}

// ForSession: counters accumulate incrementally across calls; ctx = the LAST
// message's in+cache; rate covers the recent window.
func TestForSessionIncremental(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "-x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "sess-1.jsonl")
	now := time.Date(2026, 7, 12, 10, 10, 0, 0, time.UTC)
	l1 := asst("2026-07-12T10:04:00Z", 5, 100, 1000, 200)
	l2 := asst("2026-07-12T10:06:00Z", 2, 300, 150_000, 30_000)
	if err := os.WriteFile(log, []byte(l1+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s1, ok := ForSession("claude", "sess-1", now)
	if !ok || s1.OutTok != 100 {
		t.Fatalf("first pass = %+v %v", s1, ok)
	}
	// Append → only the delta is folded in (offset-incremental).
	f, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString(l2 + "\n")
	_ = f.Close()
	s2, ok := ForSession("claude", "sess-1", now)
	if !ok || s2.OutTok != 400 || s2.InTok != 7 {
		t.Fatalf("second pass totals = %+v", s2)
	}
	// ctx = last msg in+cache_read+cache_creation = 2+150000+30000 = 180002 → 200k tier
	if s2.CtxTok != 180_002 || s2.CtxFrac < 0.89 || s2.CtxFrac > 0.91 {
		t.Errorf("ctx = %d frac %.2f", s2.CtxTok, s2.CtxFrac)
	}
	// rate: 400 out over the window starting 10:04 → 6 min → ~66/min
	if s2.RatePerMin < 60 || s2.RatePerMin > 70 {
		t.Errorf("rate = %d", s2.RatePerMin)
	}
}

// windowFor: config override wins, then a window the log stated, then evidence.
func TestWindowFor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if w := windowFor("claude", "x", 100_000, 0); w != 200_000 {
		t.Errorf("small ctx window = %d", w)
	}
	if w := windowFor("claude", "x", 400_000, 0); w != 1_000_000 {
		t.Errorf("1M-evidence window = %d", w)
	}
	// A stated window is the agent's own answer and beats the tier guess — and
	// it is routinely NOT a tier (Codex reports 258400), which is the whole
	// reason inference cannot serve it.
	if w := windowFor("codex", "", 91_343, 258_400); w != 258_400 {
		t.Errorf("stated window = %d", w)
	}
}

func TestEvaluate(t *testing.T) {
	l := Layers{CtxWarn: 0.8, SessionOutWarn: 1000, TypeRatePerMinWarn: 500}
	h := 30 * time.Minute
	if got := Evaluate(l, h, 200_000, 0.85, 10, 0); got != "ctx 85%" {
		t.Errorf("ctx breach = %q", got)
	}
	// projection: 0.5 now, rate 4000/min over a 200k window = +2%/min → 80% in ~15m
	got := Evaluate(l, h, 200_000, 0.5, 10, 4000)
	if !strings.HasPrefix(got, "ctx→") {
		t.Errorf("ctx projection = %q", got)
	}
	// A session PAST the burn line but no longer producing is silent (standing-wake-
	// backoff §5.1): a cumulative total can never de-assert, so it may not alarm on its
	// own. Past the line AND still producing warns, with the rate — the part that can
	// change back.
	if got := Evaluate(l, h, 200_000, 0.1, 1500, 0); got != "" {
		t.Errorf("a stopped session past the burn line must be silent, got %q", got)
	}
	if got := Evaluate(l, h, 200_000, 0.1, 1500, 50); !strings.HasPrefix(got, "burn 1k, +") {
		t.Errorf("burn breach with a live rate = %q", got)
	}
	// burn projection: 900 now, 10/min → cap 1000 in ~10m ≤ 30m horizon
	if got := Evaluate(l, h, 200_000, 0.1, 900, 10); !strings.HasPrefix(got, "burn→") {
		t.Errorf("burn projection = %q", got)
	}
	// quiet when far from every layer
	if got := Evaluate(l, h, 200_000, 0.1, 10, 1); got != "" {
		t.Errorf("quiet = %q", got)
	}
}

func TestTypeRateWarn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := TypeRateWarn("claude", 40_000); got == "" {
		t.Error("summed rate over the default layer should warn")
	}
	if got := TypeRateWarn("claude", 100); got != "" {
		t.Errorf("tiny rate warned: %q", got)
	}
}

// Codex logs the session's RUNNING TOTALS on every turn, where Claude logs what
// each message cost. Reading the first shape with the second's arithmetic
// multiplies a session's burn by its turn count, so this pins the difference —
// with real records, trimmed, from a live rollout log.
func TestCodexRunningTotals(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	now := time.Now().UTC()
	line := func(ago time.Duration, totalIn, cached, totalOut, lastIn int64) string {
		return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{`+
			`"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"total_tokens":%d},`+
			`"last_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":10,"total_tokens":%d},`+
			`"model_context_window":258400}}}`+"\n",
			now.Add(-ago).Format(time.RFC3339), totalIn, cached, totalOut, totalIn+totalOut, lastIn, lastIn+10)
	}
	var b strings.Builder
	b.WriteString(`{"timestamp":"x","type":"response_item","payload":{"type":"reasoning"}}` + "\n")
	b.WriteString(line(20*time.Minute, 100_000, 60_000, 1_000, 40_000))
	b.WriteString(line(6*time.Minute, 200_000, 150_000, 3_000, 70_000))
	b.WriteString(line(1*time.Minute, 300_000, 250_000, 4_000, 91_343))
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	tail := tailMessages(path, int64(len(b.String())))
	if len(tail) != 3 {
		t.Fatalf("parsed %d token_count records, want 3", len(tail))
	}
	last := tail[len(tail)-1]
	if !last.cumulative {
		t.Fatal("codex records must be marked cumulative")
	}
	// The totals are the LAST reading, never the sum of the three (which would
	// be 8,000 output instead of 4,000).
	if last.totalOut != 4_000 {
		t.Errorf("totalOut = %d, want the last reading 4000", last.totalOut)
	}
	// Non-cached input is what is comparable to Claude's: 300k total − 250k cached.
	if last.totalIn != 50_000 {
		t.Errorf("totalIn = %d, want 50000 (total minus cached)", last.totalIn)
	}
	// The context footprint is the last TURN's input, not the session's.
	if last.ctxTokens() != 91_343 {
		t.Errorf("ctx = %d, want the last turn's 91343", last.ctxTokens())
	}
	if last.window != 258_400 {
		t.Errorf("window = %d, want the stated 258400", last.window)
	}

	// The rate differences the totals across the window rather than summing
	// them: 4000 − 1000 over the 10 minutes since the window opened.
	if r := ratePerMin(tail, now); r < 250 || r > 350 {
		t.Errorf("rate = %d, want ~300/min", r)
	}

	// And the same through ForSession, which is where the mistake would
	// actually be made: the counter path would fold all three readings in.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	day := filepath.Join(home, ".codex", "sessions", "2026", "09", "01")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "01a05bcd-6e41-7bd1-8de7-e06db5a7b4d3"
	if err := os.WriteFile(filepath.Join(day, "rollout-2026-09-01T15-09-44-"+sid+".jsonl"),
		[]byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ForSession("codex", sid, now)
	if !ok {
		t.Fatal("ForSession(codex) found no log")
	}
	if got.OutTok != 4_000 || got.InTok != 50_000 {
		t.Errorf("session totals = out %d / in %d, want 4000 / 50000 (the last reading, not a sum)",
			got.OutTok, got.InTok)
	}
	if got.Window != 258_400 {
		t.Errorf("session window = %d, want the stated 258400", got.Window)
	}
	// 91343 / 258400 ≈ 0.354 — computed against the window the log named, not a
	// tier it does not belong to.
	if got.CtxFrac < 0.34 || got.CtxFrac > 0.37 {
		t.Errorf("ctx fraction = %.3f, want ~0.354", got.CtxFrac)
	}
}

// A session whose only readings predate the window is not burning anything.
func TestCodexRateOutsideWindow(t *testing.T) {
	now := time.Now().UTC()
	old := msg{at: now.Add(-2 * time.Hour), cumulative: true, totalOut: 9_000}
	if r := ratePerMin([]msg{old}, now); r != 0 {
		t.Errorf("stale-only rate = %d, want 0", r)
	}
}

// The agent string that reaches the thresholds is the DISPLAY LABEL a session
// carries, not the registry key the config is written with. Lowercasing alone
// turned "Claude Code" into "claude code", which matched nothing — so the
// documented `{"claude": {…}}` was ignored for every Claude session there has
// ever been, while Codex worked purely because its label IS its key.
func TestLayersUseTheDocumentedKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "gtmux"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "gtmux", "usage.json"),
		[]byte(`{"claude":{"ctxWarn":0.5,"window":123456},"codex":{"ctxWarn":0.4}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		agent string
		want  float64
	}{{"Claude Code", 0.5}, {"Codex", 0.4}, {"opencode", 0.8}} {
		if got := layersFor(tc.agent).CtxWarn; got != tc.want {
			t.Errorf("layersFor(%q).CtxWarn = %v, want %v", tc.agent, got, tc.want)
		}
	}
	// The window override reaches windowFor by the same label, and outranks a
	// window the log stated.
	if w := windowFor("Claude Code", "", 10_000, 258_400); w != 123_456 {
		t.Errorf("configured window = %d, want 123456", w)
	}
}
