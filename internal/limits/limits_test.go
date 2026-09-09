package limits

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// A command failure must not delete the plan it reports. Codex's windows are free
// to read, so a failing `claude -p /usage` was allowed to return "here is Codex"
// — and, by saving that, to drop Claude's last good windows from every surface
// until the command worked again. Observed for real: `gtmux serve` runs under
// launchd with PATH=/usr/bin:/bin:/usr/sbin:/sbin, where `claude` is not found.
func TestCommandFailureKeepsTheOtherAgentsWindows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	codex := t.TempDir()
	t.Setenv("CODEX_HOME", codex)
	now := time.Unix(1_788_700_000, 0)

	day := filepath.Join(codex, "sessions", "2026", "09", "06")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"timestamp":"x","type":"event_msg","payload":{"type":"token_count",`+
		`"rate_limits":{"secondary":{"used_percent":1.0,"window_minutes":10080,"resets_at":%d}}}}`+"\n",
		now.Unix()+3600)
	if err := os.WriteFile(filepath.Join(day, "rollout-a.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	seed := Report{
		Windows: []Window{{Agent: "claude", Label: "claude week (all models)", PctUsed: 58}},
		At:      1_000_000,
	}
	save(seed)

	cfg := Config{Command: "exit 3", TTLMin: 15, NearMin: 5, NearPct: 70, WarnPct: 85}
	got, ok := Get(cfg, true, now)
	if !ok {
		t.Fatal("Get reported nothing at all")
	}
	labels := make([]string, 0, len(got.Windows))
	for _, w := range got.Windows {
		labels = append(labels, w.Label)
	}
	if len(labels) != 2 || labels[0] != "claude week (all models)" || labels[1] != "codex week" {
		t.Errorf("windows = %v, want the cached claude one PLUS the fresh codex one", labels)
	}
	// And the failure must not be written back as a fresh snapshot, or the command
	// would not be retried for a whole TTL.
	if again, _ := Load(); again.At != seed.At || len(again.Windows) != 1 {
		t.Errorf("a failed refresh was saved: %+v", again)
	}
}

// The command runs through the user's LOGIN shell, not /bin/sh. On macOS /bin/sh
// is bash in POSIX mode, which reads ~/.profile only — never ~/.bash_profile or
// ~/.bashrc, where a PATH addition actually lives. Under launchd, where serve's
// own PATH is /usr/bin:/bin:/usr/sbin:/sbin, that made the command unrunnable.
func TestLoginShell(t *testing.T) {
	// A real executable in a temp dir, not a hardcoded /bin/zsh: this test also runs
	// on CI, where that path does not exist, and the check below is exactly the one
	// that would reject it.
	shell := filepath.Join(t.TempDir(), "myshell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	if got := loginShell(); got != shell {
		t.Errorf("loginShell = %q, want $SHELL", got)
	}
	// Unset, or set to something that is not an executable file, falls back to a
	// shell that reads more than /bin/sh does — never to /bin/sh itself.
	t.Setenv("SHELL", "")
	if got := loginShell(); got != "/bin/bash" {
		t.Errorf("unset = %q", got)
	}
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "not-a-shell"))
	if got := loginShell(); got != "/bin/bash" {
		t.Errorf("bogus = %q", got)
	}
}

// End to end through the real shell: the command is found the way a user's own
// shell would find it, not the way /bin/sh would.
func TestRunAndParseUsesTheLoginShell(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	script := filepath.Join(dir, "faux-usage")
	body := "#!/bin/sh\necho 'Current session: 11% used · resets Jul 13 at 1:30am'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	// A shell whose startup file puts the script's directory on PATH — the shape a
	// real machine has, and the one /bin/sh would not pick up.
	rc := filepath.Join(dir, "rc")
	if err := os.WriteFile(rc, []byte("export PATH="+dir+":$PATH\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "shell")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\n. "+rc+"\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shim)
	wins, err := runAndParse("faux-usage", DefaultConfig)
	if err != nil || len(wins) != 1 || wins[0].PctUsed != 11 {
		t.Fatalf("runAndParse = %+v, err %v — the login shell's PATH was not used", wins, err)
	}
}

// --- retry backoff ----------------------------------------------------------
//
// 2026-09-07: an upstream outage made `claude -p /usage` fail for 47 minutes, and
// gtmux answered by running it 94 times — roughly one every ten seconds — because a
// failed refresh never advances the cache's `At`, so the cache is stale by
// construction and the NEXT caller refreshes again. Every run is a real headless
// agent session, so the machine spent the outage spawning sessions to ask how much
// quota it had left.
//
// These tests count actual command runs, not internal state: the command appends a
// line to a file, so the count is what the shell really did.

// countingCmd returns a shell command that records each run and then succeeds or
// fails, plus a func reporting how many times it ran.
func countingCmd(t *testing.T, ok bool) (string, func() int) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "runs")
	tail := "false"
	if ok {
		tail = "printf '%s\\n' 'Current week (all models): 58% used · resets Jul 17 at 10:59pm'"
	}
	return "echo x >> " + log + "; " + tail, func() int {
		b, err := os.ReadFile(log)
		if err != nil {
			return 0
		}
		return strings.Count(string(b), "x")
	}
}

func TestFailingCommandIsNotRetriedOnEveryCall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	save(Report{Windows: []Window{{Label: "claude week", PctUsed: 58, Agent: "claude"}}, At: 1_000_000})
	cmd, runs := countingCmd(t, false)
	cfg := Config{Command: cmd, TTLMin: 15, TimeoutSec: 30}

	now := time.Unix(2_000_000, 0) // cache far past its TTL
	// Ten polls in the same second — serve, the menu-bar app and a phone request all
	// call Get, and during the incident they each got a fresh spawn.
	for i := 0; i < 10; i++ {
		if _, ok := Get(cfg, false, now); !ok {
			t.Fatal("Get lost the last good cache while failing")
		}
	}
	if n := runs(); n != 1 {
		t.Errorf("the command ran %d times for 10 polls in one second, want 1", n)
	}
}

func TestBackoffGrowsAndThenRetries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	save(Report{Windows: []Window{{Label: "claude week", PctUsed: 58, Agent: "claude"}}, At: 1_000_000})
	cmd, runs := countingCmd(t, false)
	cfg := Config{Command: cmd, TTLMin: 15, TimeoutSec: 30}
	base := time.Unix(2_000_000, 0)

	Get(cfg, false, base) // 1st failure → wait 1 min
	if n := runs(); n != 1 {
		t.Fatalf("first call ran %d times, want 1", n)
	}
	Get(cfg, false, base.Add(30*time.Second)) // inside the 1-minute wait
	if n := runs(); n != 1 {
		t.Errorf("retried after 30s while backing off 1m (%d runs)", n)
	}
	Get(cfg, false, base.Add(61*time.Second)) // 2nd failure → wait 2 min
	if n := runs(); n != 2 {
		t.Fatalf("did not retry after the 1-minute wait (%d runs)", n)
	}
	Get(cfg, false, base.Add(2*time.Minute)) // inside the 2-minute wait
	if n := runs(); n != 2 {
		t.Errorf("retried after 59s while backing off 2m (%d runs)", n)
	}
}

func TestBackoffIsCappedAtTheNormalTTL(t *testing.T) {
	// A broken command must cost no MORE than a working one — and no less often
	// either, so a recovery is noticed within one ordinary refresh.
	cfg := Config{Command: "false", TTLMin: 15}
	for _, fails := range []int{4, 10, 500} {
		if got := backoffFor(fails, cfg); got != 15*time.Minute {
			t.Errorf("backoffFor(%d) = %v, want the 15m TTL", fails, got)
		}
	}
	if got := backoffFor(0, cfg); got != 0 {
		t.Errorf("backoffFor(0) = %v, want 0 — a healthy cache never backs off", got)
	}
	// A short TTL caps the early steps too, rather than backing off longer than the
	// refresh interval it is protecting.
	if got := backoffFor(3, Config{TTLMin: 2}); got != 2*time.Minute {
		t.Errorf("backoffFor(3) with a 2m TTL = %v, want 2m", got)
	}
}

func TestOneSuccessClearsTheBackoff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bad, badRuns := countingCmd(t, false)
	good, _ := countingCmd(t, true)
	base := time.Unix(2_000_000, 0)

	// Three failures — deep enough to be waiting 5 minutes.
	Get(Config{Command: bad, TTLMin: 15}, false, base)
	Get(Config{Command: bad, TTLMin: 15}, false, base.Add(2*time.Minute))
	Get(Config{Command: bad, TTLMin: 15}, false, base.Add(5*time.Minute))
	if n := badRuns(); n != 3 {
		t.Fatalf("setup ran the failing command %d times, want 3", n)
	}
	if r, _ := Load(); r.Fails != 3 {
		t.Fatalf("Fails = %d after three failures, want 3", r.Fails)
	}

	if _, ok := Get(Config{Command: good, TTLMin: 15}, false, base.Add(11*time.Minute)); !ok {
		t.Fatal("the recovered command did not produce a report")
	}
	r, _ := Load()
	if r.Fails != 0 {
		t.Errorf("Fails = %d after a success, want 0 — the backoff outlived the outage", r.Fails)
	}
	if r.At != base.Add(11*time.Minute).Unix() {
		t.Errorf("At = %d, want the success time", r.At)
	}
}

func TestAFailureNeverAdvancesTheSuccessClock(t *testing.T) {
	// The reason the failure path skips the save in the first place: an `At` written
	// from a failure would cache a bad reading as fresh for the whole TTL. Recording
	// the ATTEMPT must not quietly reintroduce that.
	t.Setenv("HOME", t.TempDir())
	seed := Report{Windows: []Window{{Label: "claude week", PctUsed: 58}}, At: 1_000_000}
	save(seed)
	now := time.Unix(2_000_000, 0)
	Get(Config{Command: "false", TTLMin: 15}, false, now)

	r, ok := Load()
	if !ok || r.At != seed.At {
		t.Errorf("At = %d, want the untouched %d", r.At, seed.At)
	}
	if len(r.Windows) != 1 || r.Windows[0].PctUsed != 58 {
		t.Errorf("the failure rewrote the cached windows: %+v", r.Windows)
	}
	if r.TryAt != now.Unix() || r.Fails != 1 {
		t.Errorf("attempt bookkeeping = (try %d, fails %d), want (%d, 1)", r.TryAt, r.Fails, now.Unix())
	}
}

func TestBackoffHoldsAcrossProcesses(t *testing.T) {
	// serve, the menu-bar app and the CLI each call Get from their own process and
	// share nothing but this file — so the backoff has to live in it, not in memory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd, runs := countingCmd(t, false)
	cfg := Config{Command: cmd, TTLMin: 15}
	now := time.Unix(2_000_000, 0)

	Get(cfg, false, now)
	// A "second process" reads the same state from disk — which is all Get keeps.
	if r, ok := Load(); !ok || r.Fails != 1 {
		t.Fatalf("the attempt was not persisted: %+v", r)
	}
	Get(cfg, false, now.Add(10*time.Second))
	if n := runs(); n != 1 {
		t.Errorf("a second caller re-ran the command (%d runs) — backoff did not survive", n)
	}
}

func TestForceIgnoresTheBackoff(t *testing.T) {
	// `gtmux limits --force` is a person asking on purpose; making them wait out a
	// backoff they cannot see would be its own bug.
	t.Setenv("HOME", t.TempDir())
	cmd, runs := countingCmd(t, false)
	cfg := Config{Command: cmd, TTLMin: 15}
	now := time.Unix(2_000_000, 0)

	Get(cfg, false, now)
	Get(cfg, true, now.Add(time.Second))
	if n := runs(); n != 2 {
		t.Errorf("--force ran the command %d times, want 2 (it must bypass the backoff)", n)
	}
}

// --- command timeout --------------------------------------------------------

func TestSlowCommandIsAbandoned(t *testing.T) {
	// The command had no bound at all: a `claude` that never answered was waited on
	// forever, while the next poll started another beside it.
	t.Setenv("HOME", t.TempDir())
	cfg := Config{Command: "sleep 30", TTLMin: 15, TimeoutSec: 1}
	start := time.Now()
	_, err := runAndParse(cfg.Command, cfg)
	if err == nil {
		t.Error("a command that outran its timeout returned no error")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("runAndParse took %v — it waited on the hung command", d)
	}
}

func TestTimeoutDefaultsRatherThanMeaningNoLimit(t *testing.T) {
	// A zero-valued Config — or one unmarshalled from a cache written before this
	// field existed — must not read as "wait forever".
	if got := commandTimeout(Config{}); got != time.Duration(DefaultConfig.TimeoutSec)*time.Second {
		t.Errorf("commandTimeout(zero Config) = %v, want the default", got)
	}
	if got := commandTimeout(Config{TimeoutSec: 5}); got != 5*time.Second {
		t.Errorf("commandTimeout = %v, want 5s", got)
	}
}

// A Codex reading whose windows have all ended must SAY SO, not vanish.
//
// This is the 2026-09-07 report: Codex rows were on the usage screen one day and gone
// the next, with nothing to distinguish "your plan is fine" from "gtmux stopped
// reading it". Codex reports passively — it writes its windows into a session log when
// it takes a turn — so a day without Codex is enough for its last reading to roll over.
func writeCodexReading(t *testing.T, home string, mtime time.Time, primaryReset, secondaryReset int64) {
	t.Helper()
	day := filepath.Join(home, "sessions", "2026", "09", "06")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	line := fmt.Sprintf(`{"timestamp":"2026-09-06T03:03:33.706Z","type":"event_msg","payload":{`+
		`"type":"token_count","info":{"total_token_usage":{"input_tokens":1,"output_tokens":1}},`+
		`"rate_limits":{"limit_id":"codex","primary":{"used_percent":0.0,"window_minutes":300,"resets_at":%d},`+
		`"secondary":{"used_percent":1.0,"window_minutes":10080,"resets_at":%d},"plan_type":"plus"}}}`+"\n",
		primaryReset, secondaryReset)
	p := filepath.Join(day, "rollout-2026-09-06T10-55-54-abc.jsonl")
	if err := os.WriteFile(p, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestCodexRolledOverReportsItselfInsteadOfDisappearing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	now := time.Unix(1_788_778_453, 0)
	// Both windows ended: the 5-hour one yesterday, the weekly one this morning —
	// the exact shape measured on the operator's machine.
	writeCodexReading(t, home, now.Add(-31*time.Hour), now.Unix()-90_000, now.Unix()-25_000)

	if wins, ok := codexWindows(now); ok || len(wins) != 0 {
		t.Fatalf("an ended window must not be reported as live: %+v ok=%v", wins, ok)
	}
	if !codexUnknown(now) {
		t.Fatal("Codex was used yesterday and its plan is unreadable — that has to be sayable")
	}
	got := unknownPlans(false, now)
	if len(got) != 1 || got[0].Agent != "codex" || got[0].Reason != "rolled-over" {
		t.Fatalf("unknown = %+v", got)
	}
}

func TestCodexUnusedForAWeekStaysSilent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	now := time.Unix(1_788_778_453, 0)
	// Same ended reading, but from long enough ago that this account is not on the
	// plan any more. A standing "unknown" would be a nag about a tool nobody uses.
	writeCodexReading(t, home, now.Add(-30*24*time.Hour), now.Unix()-90_000, now.Unix()-25_000)

	if codexUnknown(now) {
		t.Fatal("a month-old reading is not news")
	}
	if got := unknownPlans(false, now); len(got) != 0 {
		t.Fatalf("unknown = %+v, want none", got)
	}
}

func TestNoCodexAtAllSaysNothing(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	now := time.Unix(1_788_778_453, 0)
	if codexUnknown(now) {
		t.Fatal("an operator who does not use Codex must not be told about Codex")
	}
}

// A live Codex window must reach the report even when the CLAUDE cache is fresh.
//
// Codex is read from a local file, so there is nothing to amortise by caching it — and
// caching it was wrong twice: a window that had since ended kept being served until the
// Claude cache expired, and a reading that had rolled over could not report itself,
// because the cache-hit branch returned before anything looked at Codex.
func TestFreshClaudeCacheStillRereadsCodex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	t.Setenv("HOME", t.TempDir()) // XDG_DATA_HOME is not read; this used to write the real cache
	now := time.Unix(1_788_778_453, 0)
	cfg := Config{Command: "true", TTLMin: 15, NearPct: 90, NearMin: 5, WarnPct: 85}

	// A cache that is FRESH by Claude's clock, and that still carries a Codex row from
	// a window which has since ended.
	save(Report{
		At:    now.Unix() - 60,
		TryAt: now.Unix() - 60,
		Windows: []Window{
			{Label: "claude week (all models)", PctUsed: 50, Agent: "claude"},
			{Label: "codex week", PctUsed: 1, Agent: "codex", ResetUnix: now.Unix() - 25_000},
		},
	})
	writeCodexReading(t, home, now.Add(-31*time.Hour), now.Unix()-90_000, now.Unix()-25_000)

	r, ok := Get(cfg, false, now)
	if !ok {
		t.Fatal("report unavailable")
	}
	for _, w := range r.Windows {
		if w.Agent == "codex" {
			t.Fatalf("an ended Codex window was served from cache: %+v", w)
		}
	}
	if len(r.Unknown) != 1 || r.Unknown[0].Agent != "codex" {
		t.Fatalf("unknown = %+v, want codex", r.Unknown)
	}
}
