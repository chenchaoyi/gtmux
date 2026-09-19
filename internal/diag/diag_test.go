package diag

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// setup points the store at a temp home and pins the clock.
func setup(t *testing.T, at time.Time) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_DEBUG", "")
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
	if err := os.MkdirAll(state.LogsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	return state.LogsDir()
}

func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, MaxEntry*2), MaxEntry*2)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("a line is not a whole JSON entry: %q", sc.Text())
		}
		out = append(out, e)
	}
	return out
}

var day1 = time.Date(2026, 9, 19, 9, 36, 5, 0, time.Local)

func TestManyWritersLeaveWholeLines(t *testing.T) {
	dir := setup(t, day1)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				For("serve").Info("serve.test", "a line", "g", g, "i", i, "pad", strings.Repeat("x", 300))
			}
		}(g)
	}
	wg.Wait()
	got := readEntries(t, DayFile(dir, "2026-09-19"))
	if len(got) != 400 {
		t.Fatalf("got %d entries, want 400", len(got))
	}
	if fi, _ := os.Stat(DayFile(dir, "2026-09-19")); fi.Mode().Perm() != 0o600 {
		t.Errorf("the day file is %#o, want 0600", fi.Mode().Perm())
	}
}

func TestAnActionSaysWhoWhatAndHowItEnded(t *testing.T) {
	dir := setup(t, day1)
	For("serve").Act("act.send", "phone:3f9c20e1", "%7", OK, "typed into a pane", "bytes", 42)
	For("serve").Act("act.send", "phone:3f9c20e1", "%7", Refused, "the box held a draft", "reason", "refused-draft")
	got := readEntries(t, DayFile(dir, "2026-09-19"))
	if got[0].Kind != KindAct || got[0].Actor != "phone:3f9c20e1" || got[0].Target != "%7" || got[0].Level != "info" {
		t.Errorf("first action = %+v", got[0])
	}
	if got[1].Outcome != Refused || got[1].Level != "warn" {
		t.Errorf("a refused action should be a warning so --level warn shows it: %+v", got[1])
	}
}

// The serve log held the master token 398 times. Whatever a call site passes, the store
// never holds a registered secret or a credential-shaped value.
func TestCredentialsNeverReachTheStore(t *testing.T) {
	dir := setup(t, day1)
	const tok = "632c9e5f0e74f6bfc02426bd1c7bb089"
	RegisterSecret(tok)
	lg := For("serve")
	lg.Warn("auth.rejected", "bad token "+tok, "header", "Bearer "+tok, "device_token", "abc12345")
	lg.Act("act.pair", "user", "http://127.0.0.1:8765/#c=4ff9894607e2fd16", OK, "link http://x/#g=a.b-c~d",
		"code", "4ff9894607e2fd16", "exit_code", 2, "note", "authorization: Basic Zm9vOmJhcg==")
	lg.Info("serve.start", "started", "url", tok)

	b, _ := os.ReadFile(DayFile(dir, "2026-09-19"))
	for _, leak := range []string{tok, "4ff9894607e2fd16", "abc12345", "a.b-c~d", "Zm9vOmJhcg=="} {
		if strings.Contains(string(b), leak) {
			t.Errorf("the store holds %q:\n%s", leak, b)
		}
	}
	if !strings.Contains(string(b), `"exit_code":2`) {
		t.Error("exit_code is not a credential and should survive")
	}
}

func TestAnEntryNeverExceedsOneAtomicWrite(t *testing.T) {
	dir := setup(t, day1)
	For("cli").Error("cli.failed", strings.Repeat("é", 5000), "out", strings.Repeat("y", 9000))
	b, _ := os.ReadFile(DayFile(dir, "2026-09-19"))
	if len(b) > MaxEntry {
		t.Errorf("an entry is %d bytes, over the %d an atomic append allows", len(b), MaxEntry)
	}
	got := readEntries(t, DayFile(dir, "2026-09-19"))
	if !got[0].Truncated {
		t.Error("a cut entry does not say it was cut")
	}
}

func touchDay(t *testing.T, dir, name string, size int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Repeat("x", size)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Cleanup does not wait for serve: the first entry of a new day runs it, whoever writes.
func TestTheFirstEntryOfADayCleansTheStore(t *testing.T) {
	dir := setup(t, day1)
	touchDay(t, dir, "2026-08-10.jsonl", 100) // 40 days old
	touchDay(t, dir, "2026-09-01.jsonl", 100) // inside the window
	For("hook").Info("hook.fired", "a hook ran")

	if _, err := os.Stat(filepath.Join(dir, "2026-08-10.jsonl")); !os.IsNotExist(err) {
		t.Error("a 40-day-old file survived the new day's cleanup")
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-09-01.jsonl")); err != nil {
		t.Error("a file inside the retention window was removed")
	}
	var sawCleanup bool
	for _, e := range readEntries(t, DayFile(dir, "2026-09-19")) {
		if e.Event == "act.cleanup" && e.Target == "2026-08-10.jsonl" && e.Attrs["reason"] == "expired" {
			sawCleanup = true
		}
	}
	if !sawCleanup {
		t.Error("the removal was not recorded as an act.cleanup entry")
	}
}

func TestOverTheCapTheOldestDaysGoFirstAndTodayStays(t *testing.T) {
	dir := setup(t, day1)
	cfg := filepath.Join(state.ConfigDir(), "config.json")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o700)
	_ = os.WriteFile(cfg, []byte(`{"logs":{"maxMB":1}}`), 0o600)
	touchDay(t, dir, "2026-09-10.jsonl", 600<<10)
	touchDay(t, dir, "2026-09-15.jsonl", 600<<10)
	touchDay(t, dir, "2026-09-19.jsonl", 600<<10)

	removed := Cleanup()
	if len(removed) != 2 {
		t.Fatalf("removed %d files, want the two older days: %+v", len(removed), removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-09-19.jsonl")); err != nil {
		t.Error("today's file was removed to get under the cap")
	}
}

// A loop that floods one day rotates within the day and says who did it.
func TestARunawayWriterIsNamed(t *testing.T) {
	dir := setup(t, day1)
	f, _ := os.OpenFile(DayFile(dir, "2026-09-19"), os.O_CREATE|os.O_WRONLY, 0o600)
	_ = f.Truncate(SegmentCap - (2 << 20)) // sparse bulk
	_, _ = f.Seek(0, 2)
	line := `{"component":"tunnel","event":"tunnel.probe.failed"}` + "\n"
	for w := 0; w < (2<<20)/len(line)+1; w++ {
		_, _ = f.WriteString(line)
	}
	_ = f.Close()

	For("serve").Info("serve.test", "after the cap")
	next := filepath.Join(dir, "2026-09-19.1.jsonl")
	got := readEntries(t, next)
	if len(got) < 2 || got[0].Event != "log.runaway" {
		t.Fatalf("no runaway entry opened the next segment: %+v", got)
	}
	if got[0].Attrs["component"] != "tunnel" || got[0].Attrs["event"] != "tunnel.probe.failed" {
		t.Errorf("the runaway entry names %v/%v, want tunnel/tunnel.probe.failed", got[0].Attrs["component"], got[0].Attrs["event"])
	}
	For("serve").Info("serve.test", "again")
	runaways := 0
	for _, e := range readEntries(t, next) {
		if e.Event == "log.runaway" {
			runaways++
		}
	}
	if runaways != 1 {
		t.Errorf("the runaway was recorded %d times, want once", runaways)
	}
}

func TestPastTheDailyCeilingDebugIsDropped(t *testing.T) {
	dir := setup(t, day1)
	t.Setenv("GTMUX_DEBUG", "serve")
	for seg := 0; seg < 3; seg++ {
		p := segmentPath(dir, "2026-09-19", seg)
		f, _ := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o600)
		_ = f.Truncate(SegmentCap)
		_ = f.Close()
	}
	For("serve").Debug("serve.trace", "noise")
	For("serve").Warn("serve.real", "still recorded")
	var sawReal, sawNoise bool
	for _, sf := range Files(dir) {
		b, _ := os.ReadFile(sf.Path)
		sawReal = sawReal || strings.Contains(string(b), "serve.real")
		sawNoise = sawNoise || strings.Contains(string(b), "serve.trace")
	}
	if sawNoise || !sawReal {
		t.Errorf("past the ceiling: debug written=%v, warn written=%v; want false, true", sawNoise, sawReal)
	}
}

func TestDebugIsOffUnlessAsked(t *testing.T) {
	dir := setup(t, day1)
	For("serve").Debug("serve.trace", "hidden")
	t.Setenv("GTMUX_HOOK_DEBUG", "1") // the old name still works, for its component
	For("hook").Debug("hook.trace", "shown")
	For("serve").Debug("serve.trace2", "still hidden")
	t.Setenv("GTMUX_DEBUG", "all")
	For("serve").Debug("serve.trace3", "shown")
	b, _ := os.ReadFile(DayFile(dir, "2026-09-19"))
	for ev, want := range map[string]bool{"serve.trace": false, "hook.trace": true, "serve.trace2": false, "serve.trace3": true} {
		if got := strings.Contains(string(b), `"event":"`+ev+`"`); got != want {
			t.Errorf("%s written=%v, want %v", ev, got, want)
		}
	}
}

func TestStatusKeepsItsSinceAndGoesStale(t *testing.T) {
	setup(t, day1)
	Publish("tunnel", "connected", 90*time.Second, map[string]any{"backend": "direct"})
	now = func() time.Time { return day1.Add(40 * time.Second) }
	Publish("tunnel", "connected", 90*time.Second, map[string]any{"backend": "direct"})

	st, fresh := ReadStatus("tunnel")
	if !fresh || st.State != "connected" {
		t.Fatalf("status = %+v fresh=%v", st, fresh)
	}
	if !st.Since.Equal(day1) {
		t.Errorf("since moved to %v on a heartbeat; it should stay at %v", st.Since, day1)
	}
	if fi, _ := os.Stat(StatusPath("tunnel")); fi.Mode().Perm() != 0o600 {
		t.Errorf("status file is %#o, want 0600", fi.Mode().Perm())
	}

	now = func() time.Time { return day1.Add(10 * time.Minute) }
	if _, fresh := ReadStatus("tunnel"); fresh {
		t.Error("a status its writer stopped updating ten minutes ago still reads as fresh")
	}
	Publish("tunnel", "down", 90*time.Second, nil)
	if st, _ := ReadStatus("tunnel"); !st.Since.Equal(day1.Add(10 * time.Minute)) {
		t.Errorf("a new state kept the old since: %v", st.Since)
	}
}

func TestParseLevelAndString(t *testing.T) {
	for _, l := range []Level{Debug, Info, Warn, Error} {
		if ParseLevel(l.String()) != l {
			t.Errorf("round trip of %v failed", l)
		}
	}
	if ParseLevel("nonsense") != Info {
		t.Error("an unknown level should read as info")
	}
}
