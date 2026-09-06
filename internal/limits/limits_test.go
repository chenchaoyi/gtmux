package limits

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Get must return the LAST GOOD cache (ok=true) when the command fails or yields no
// windows, never a blank Report — otherwise a transient `/usage` hiccup silently blanks
// the usage/limits surface (mobile + menu-bar) and drops HQ's plan-exhaustion warning.
// This is the one orchestration path in Get that runAndParse/parse tests don't reach.
func TestGetKeepsLastGoodCacheOnFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolates state.Dir() → cachePath()
	seed := Report{Windows: []Window{{Label: "week (all models)", PctUsed: 58}}, At: 1_000_000, Warn: ""}
	save(seed)

	now := time.Unix(2_000_000, 0) // far past the seed's TTL → force a refresh attempt

	// Command exits non-zero → runAndParse errors → keep the seed, ok stays true.
	if r, ok := Get(Config{Command: "false", TTLMin: 15}, true, now); !ok || len(r.Windows) != 1 || r.Windows[0].PctUsed != 58 {
		t.Errorf("failing command: Get = (%+v, %v), want the seeded 58%% window with ok=true", r, ok)
	}
	// Command succeeds but prints nothing → 0 windows → also keep the seed (not blank).
	if r, ok := Get(Config{Command: "true", TTLMin: 15}, true, now); !ok || len(r.Windows) != 1 {
		t.Errorf("empty output: Get = (%+v, %v), want the seed held, not a blank report", r, ok)
	}
	// No cache at all + failing command → blank + ok=false (nothing to show, honestly).
	t.Setenv("HOME", t.TempDir()) // fresh HOME → no cache
	if r, ok := Get(Config{Command: "false", TTLMin: 15}, true, now); ok || len(r.Windows) != 0 {
		t.Errorf("no cache + failure: Get = (%+v, %v), want empty + ok=false", r, ok)
	}
}

const sample = `You are currently using your subscription to power your Claude Code usage

Current session: 11% used · resets Jul 13 at 1:30am (Asia/Shanghai)
Current week (all models): 58% used · resets Jul 17 at 10:59pm (Asia/Shanghai)
Current week (Fable): 88% used · resets Jul 17 at 10:59pm (Asia/Shanghai)

What's contributing to your limits usage?
  99% of your usage came from subagent-heavy sessions
  93% of your usage was at >150k context
`

func TestParse(t *testing.T) {
	w := parse(sample)
	if len(w) != 3 {
		t.Fatalf("want 3 windows, got %d: %+v", len(w), w)
	}
	if w[0].Label != "session" || w[0].PctUsed != 11 || w[0].ResetAt != "Jul 13 at 1:30am" {
		t.Errorf("session window = %+v", w[0])
	}
	if w[1].Label != "week (all models)" || w[1].PctUsed != 58 {
		t.Errorf("week window = %+v", w[1])
	}
	if w[2].Label != "week (fable)" || w[2].PctUsed != 88 {
		t.Errorf("model window = %+v", w[2])
	}
	// the "99% of your usage" prose must NOT parse as a window
	for _, x := range w {
		if x.PctUsed == 99 || x.PctUsed == 93 {
			t.Errorf("prose leaked into windows: %+v", x)
		}
	}
}

func TestParseGarbled(t *testing.T) {
	if w := parse("total nonsense\nno percentages here"); len(w) != 0 {
		t.Errorf("garbled → %+v, want none", w)
	}
}

func TestWarnOf(t *testing.T) {
	wins := []Window{
		{Label: "session", PctUsed: 95}, // session excluded even at 95%
		{Label: "week (all models)", PctUsed: 58},
		{Label: "week (fable)", PctUsed: 88},
	}
	if got := warnOf(wins, 85); got != "week (fable) 88%" {
		t.Errorf("warnOf = %q", got)
	}
	if got := warnOf(wins, 90); got != "" {
		t.Errorf("warnOf(90) = %q, want empty", got)
	}
}

func TestTTLNearAware(t *testing.T) {
	cfg := DefaultConfig
	calm := Report{Windows: []Window{{Label: "week", PctUsed: 40}}}
	if ttl(calm, cfg) != 15*time.Minute {
		t.Errorf("calm ttl = %v", ttl(calm, cfg))
	}
	near := Report{Windows: []Window{{Label: "week", PctUsed: 72}}}
	if ttl(near, cfg) != 5*time.Minute {
		t.Errorf("near ttl = %v", ttl(near, cfg))
	}
}

func TestFresh(t *testing.T) {
	cfg := DefaultConfig
	now := time.Unix(1_000_000, 0)
	r := Report{Windows: []Window{{PctUsed: 40}}, At: now.Add(-10 * time.Minute).Unix()}
	if !Fresh(r, cfg, now) {
		t.Error("10m-old calm cache should be fresh (15m TTL)")
	}
	r.At = now.Add(-20 * time.Minute).Unix()
	if Fresh(r, cfg, now) {
		t.Error("20m-old cache should be stale")
	}
	// near-cap shortens the TTL to 5m
	rn := Report{Windows: []Window{{PctUsed: 80}}, At: now.Add(-7 * time.Minute).Unix()}
	if Fresh(rn, cfg, now) {
		t.Error("7m-old near-cap cache should be stale (5m TTL)")
	}
}

// Codex records its rate limits into the session rollout, so the windows come
// from the log rather than a command. These pin the two rules that measurement
// forced (see codex.go): the window is named by its DURATION, and a reading that
// outlived its own window is dropped.
func TestCodexWindowsFromLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	day := filepath.Join(home, "sessions", "2026", "09", "06")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_788_700_000, 0)
	live := now.Unix() + 3600    // still running
	expired := now.Unix() - 3600 // reset already happened
	// The real shape, from a live session on 2026-09-06.
	line := fmt.Sprintf(`{"timestamp":"2026-09-06T03:03:33.706Z","type":"event_msg","payload":{`+
		`"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"output_tokens":1}},`+
		`"rate_limits":{"limit_id":"codex","primary":{"used_percent":0.0,"window_minutes":300,"resets_at":%d},`+
		`"secondary":{"used_percent":86.0,"window_minutes":10080,"resets_at":%d},"plan_type":"plus"}}}`+"\n",
		live, live)
	if err := os.WriteFile(filepath.Join(day, "rollout-2026-09-06T10-55-54-abc.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	wins, ok := codexWindows(now)
	if !ok || len(wins) != 2 {
		t.Fatalf("windows = %+v ok=%v, want 2", wins, ok)
	}
	// Named by duration, and tagged with whose plan it is.
	if wins[0].Label != "codex session" || wins[1].Label != "codex week" {
		t.Errorf("labels = %q / %q", wins[0].Label, wins[1].Label)
	}
	if wins[0].Agent != "codex" || wins[1].ResetUnix != live {
		t.Errorf("agent/reset not carried: %+v", wins)
	}
	// A weekly window over the threshold warns, and says whose it is — the spawn
	// preflight acts on this string.
	if w := warnOf(wins, 85); w != "codex week 86%" {
		t.Errorf("warn = %q", w)
	}

	// The SAME reading, once its windows have reset, is not a current answer.
	stale := fmt.Sprintf(`{"timestamp":"x","type":"event_msg","payload":{"type":"token_count",`+
		`"rate_limits":{"primary":{"used_percent":3.0,"window_minutes":300,"resets_at":%d},`+
		`"secondary":{"used_percent":3.0,"window_minutes":10080,"resets_at":%d}}}}`+"\n", expired, expired)
	if err := os.WriteFile(filepath.Join(day, "rollout-2026-09-06T10-55-54-abc.jsonl"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if wins, ok := codexWindows(now); ok {
		t.Errorf("expired windows reported as current: %+v", wins)
	}
}

// `primary` is a POSITION, not an identity: on this machine 41 blocks carry the
// weekly window there. Naming by position would label a week "session".
func TestCodexWindowNamedByDuration(t *testing.T) {
	now := time.Unix(1_788_700_000, 0)
	weekly := &codexWindow{UsedPercent: 12, WindowMin: 10080, ResetsAt: now.Unix() + 60}
	got := codexToWindows(weekly, nil, now)
	if len(got) != 1 || got[0].Label != "codex week" {
		t.Errorf("primary-holding-a-week = %+v, want codex week", got)
	}
}

// Every window says whose plan it is, the first agent's included. A bare
// "session" beside "codex session" reads as the general case with a special case
// next to it, and the label is the only field most renderers show.
func TestEveryWindowIsQualified(t *testing.T) {
	if got := qualify("claude", "week (all models)"); got != "claude week (all models)" {
		t.Errorf("claude label = %q", got)
	}
	// Idempotent: qualifying twice must not stutter.
	if got := qualify("codex", "codex week"); got != "codex week" {
		t.Errorf("re-qualified = %q", got)
	}
	// The warn string is what the spawn preflight prints, so it names the plan.
	wins := []Window{{Label: qualify("claude", "week (all models)"), PctUsed: 90}}
	if w := warnOf(wins, 85); w != "claude week (all models) 90%" {
		t.Errorf("warn = %q", w)
	}

	// And through Get, which is where the qualifying is actually wired: the
	// parser reports what the command said, Get is what says whose it is.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir()) // no codex rollouts → command windows only
	cfg := Config{Command: `printf '%s\n' "Current session: 11% used · resets Jul 13 at 1:30am"`,
		TTLMin: 15, NearMin: 5, NearPct: 70, WarnPct: 85}
	r, ok := Get(cfg, true, time.Unix(1_788_700_000, 0))
	if !ok || len(r.Windows) != 1 {
		t.Fatalf("Get = %+v ok=%v", r.Windows, ok)
	}
	if r.Windows[0].Label != "claude session" || r.Windows[0].Agent != "claude" {
		t.Errorf("Get window = %+v, want a qualified claude session", r.Windows[0])
	}
}

// A summary line has room for one window per plan, so it keeps the tightest —
// the one that runs out first, which is what "where do I stand" asks.
func TestSummaryKeepsTheTightestPerPlan(t *testing.T) {
	wins := []Window{
		{Agent: "claude", Label: "claude session", PctUsed: 12},
		{Agent: "claude", Label: "claude week (all models)", PctUsed: 18},
		{Agent: "claude", Label: "claude week (fable)", PctUsed: 11},
		{Agent: "codex", Label: "codex session", PctUsed: 0},
		{Agent: "codex", Label: "codex week", PctUsed: 1},
	}
	got := Summary(wins)
	if len(got) != 2 {
		t.Fatalf("summary = %+v, want one per plan", got)
	}
	if got[0].Label != "claude week (all models)" || got[1].Label != "codex week" {
		t.Errorf("summary kept %q / %q", got[0].Label, got[1].Label)
	}
	// Unlike warnOf, a session window is eligible: at 95% it IS where you stand.
	wins[0].PctUsed = 95
	if got := Summary(wins); got[0].Label != "claude session" {
		t.Errorf("tightest = %q, want the session window", got[0].Label)
	}
	// An older serve sends no agent field; nothing is merged away.
	old := []Window{{Label: "session", PctUsed: 12}, {Label: "week (all models)", PctUsed: 18}}
	if got := Summary(old); len(got) != 2 {
		t.Errorf("unqualified windows collapsed: %+v", got)
	}
}
