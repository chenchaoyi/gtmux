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

// windowFor: config override wins; else the smallest tier ≥ observed.
func TestWindowFor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if w := windowFor("claude", "x", 100_000); w != 200_000 {
		t.Errorf("small ctx window = %d", w)
	}
	if w := windowFor("claude", "x", 400_000); w != 1_000_000 {
		t.Errorf("1M-evidence window = %d", w)
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
	if got := Evaluate(l, h, 200_000, 0.1, 1500, 0); got != "burn 1k" {
		t.Errorf("burn breach = %q", got)
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
