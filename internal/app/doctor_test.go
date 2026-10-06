package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// TestRenderSectionsTally checks the ok/recommended/blocking tally across status
// levels (stInfo must count toward none).
func TestRenderSectionsTally(t *testing.T) {
	secs := []dsection{{"x", []dcheck{
		{stOK, "a", "", ""},
		{stOK, "b", "", ""},
		{stRec, "c", "", ""},
		{stMiss, "d", "", ""},
		{stInfo, "e", "", ""},
	}}}
	ok, rec, miss := renderSections(secs)
	if ok != 2 || rec != 1 || miss != 1 {
		t.Fatalf("tally = ok %d rec %d miss %d, want 2/1/1", ok, rec, miss)
	}
}

func TestAdvisoryRemaining(t *testing.T) {
	// advisoryRemaining keeps only the still-flagged rows (recommend + blocking), in
	// order — the items --fix's "Nothing was changed" used to hide (e.g. HQ session
	// health). OK / info rows are dropped.
	secs := []dsection{
		{"tmux", []dcheck{{stOK, "locale", "", ""}, {stInfo, "config", "", ""}}},
		{"HQ", []dcheck{
			{stOK, "board", "", ""},
			{stRec, "HQ conversation health", "ctx 20% · 24h", "over age 24h — `gtmux hq --rotate`"},
		}},
		{"x", []dcheck{{stMiss, "tmux", "", "install it"}}},
	}
	got := advisoryRemaining(secs)
	if len(got) != 2 {
		t.Fatalf("want 2 flagged rows, got %d: %+v", len(got), got)
	}
	if got[0].label != "HQ conversation health" || got[1].label != "tmux" {
		t.Fatalf("wrong rows/order: %q, %q", got[0].label, got[1].label)
	}
	// All-OK sections yield nothing (the "everything's already set" branch).
	if n := len(advisoryRemaining([]dsection{{"y", []dcheck{{stOK, "a", "", ""}, {stInfo, "b", "", ""}}}})); n != 0 {
		t.Fatalf("all-OK should yield 0 remaining, got %d", n)
	}
}

// TestIsUTF8Locale covers the charset sniff used by rowLocale / stepLocale.
func TestIsUTF8Locale(t *testing.T) {
	for _, v := range []string{"en_US.UTF-8", "zh_CN.UTF-8", "C.utf8", "en_US.utf-8"} {
		if !isUTF8Locale(v) {
			t.Errorf("%q should be UTF-8", v)
		}
	}
	for _, v := range []string{"", "C", "POSIX", "en_US", "en_US.ISO8859-1"} {
		if isUTF8Locale(v) {
			t.Errorf("%q should not be UTF-8", v)
		}
	}
}

// withLocaleEnv stands in for the tmux server's global environment.
func withLocaleEnv(t *testing.T, env map[string]string, server, known bool) {
	t.Helper()
	saved := localeEnv
	localeEnv = func() (map[string]string, bool, bool) { return env, server, known }
	t.Cleanup(func() { localeEnv = saved })
}

// The locale a new pane starts with is the tmux server's (LC_ALL > LC_CTYPE > LANG), not
// gtmux's own: a UTF-8 shell over a C server read fine, and the reverse read as a
// problem (%12, 2026-10-06). This process's environment is set to the opposite of the
// server's in every case, so reading it would give the wrong answer.
func TestRowLocaleReadsTheServer(t *testing.T) {
	t.Setenv("GTMUX_LANG", "en")
	for _, tc := range []struct {
		name   string
		own    string // this process's LANG, deliberately the opposite
		env    map[string]string
		server bool
		known  bool
		want   int
	}{
		{"UTF-8 shell, C server", "en_US.UTF-8", map[string]string{"LANG": "C"}, true, true, stRec},
		{"C shell, UTF-8 server", "C", map[string]string{"LANG": "en_US.UTF-8"}, true, true, stOK},
		{"both UTF-8", "en_US.UTF-8", map[string]string{"LANG": "en_US.UTF-8"}, true, true, stOK},
		{"both C", "C", map[string]string{"LANG": "C"}, true, true, stRec},
		{"LC_ALL outranks a UTF-8 LANG", "en_US.UTF-8", map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, true, true, stRec},
		{"nothing set", "en_US.UTF-8", map[string]string{}, true, true, stRec},
		{"no server: this shell", "C", map[string]string{"LANG": "en_US.UTF-8"}, false, true, stOK},
		{"server could not be asked", "C", nil, false, false, stInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LANG", tc.own)
			withLocaleEnv(t, tc.env, tc.server, tc.known)
			if got := rowLocale(); got.status != tc.want {
				t.Errorf("rowLocale() = %+v, want status %v", got, tc.want)
			}
		})
	}
}

// show-environment output: NAME=value, and "-NAME" for a variable removed.
func TestParseTmuxEnv(t *testing.T) {
	env := parseTmuxEnv("LANG=en_US.UTF-8\n-LC_ALL\nTERM=xterm-256color\n")
	if env["LANG"] != "en_US.UTF-8" || env["TERM"] != "xterm-256color" {
		t.Errorf("env = %v", env)
	}
	if _, ok := env["LC_ALL"]; ok {
		t.Error("a removed variable counted as set")
	}
	if _, ok := env["-LC_ALL"]; ok {
		t.Error("the removal marker became a variable")
	}
}

// TestClaudeHookInstalled exercises the settings.json walk against a temp HOME:
// absent file, a non-gtmux hook, and a real gtmux hook command.
func TestClaudeHookInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if claudeHookInstalled() {
		t.Error("no settings.json → should report not installed")
	}

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath()

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/usr/bin/other thing"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if claudeHookInstalled() {
		t.Error("non-gtmux hook → should report not installed")
	}

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/opt/bin/gtmux hook"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !claudeHookInstalled() {
		t.Error("gtmux hook present → should report installed")
	}
}

// TestCodexNotifyIsGtmux: only a notify line referencing both gtmux and codex counts.
func TestCodexNotifyIsGtmux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if codexNotifyIsGtmux() {
		t.Error("no config.toml → not wired")
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".codex", "config.toml")

	if err := os.WriteFile(cfg, []byte(`notify = ["some-other-program"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if codexNotifyIsGtmux() {
		t.Error("unrelated notify → not wired")
	}

	if err := os.WriteFile(cfg, []byte(`notify = ["/opt/bin/gtmux", "hook", "--agent", "codex"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !codexNotifyIsGtmux() {
		t.Error("gtmux+codex notify → wired")
	}
}

// The HQ-maintenance rows are the visible half of the periodic distill/self-check
// cadence: both passes are SILENT by design, so without this "it has not distilled in
// three weeks" is indistinguishable from "nothing needed distilling". A slipped cadence
// must read as ⚠ (stRec), a healthy one as ✓, and a never-run one as a neutral note.
func TestHQMaintenanceChecks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	rows := hqMaintenanceChecks(now)
	if len(rows) != 4 {
		t.Fatalf("want 4 maintenance rows (distill, self-check, promotions, knowledge sync), got %d", len(rows))
	}
	for _, r := range rows[:2] {
		if r.status != stInfo {
			t.Errorf("%s: fresh install should be a neutral note, got status %d", r.label, r.status)
		}
	}
	// The promotions row (hq-promotion-exit) is quiet-OK on an empty queue — a
	// fresh install has nothing waiting to land, which is health, not absence.
	if rows[2].status != stOK {
		t.Errorf("empty promotions queue: status %d, want stOK", rows[2].status)
	}

	// Distill 3 days ago (inside its weekly floor) → OK. Self-check 40h ago (past the
	// daily floor + 12h grace) → needs attention.
	writeMarker(t, home, "last-distill", "9740800 42") // now - 3d
	writeMarker(t, home, "last-self-check", "9856000") // now - 40h
	rows = hqMaintenanceChecks(now)
	if rows[0].status != stRec || !strings.Contains(rows[0].note, "completion") {
		t.Errorf("unacknowledged distill: %+v, want a pending warning", rows[0])
	}
	writeMarker(t, home, "last-distill-complete", "9740860 9740800")
	rows = hqMaintenanceChecks(now)
	if rows[0].status != stOK {
		t.Errorf("completed distill 3d ago: status %d, want stOK", rows[0].status)
	}
	if rows[1].status != stRec {
		t.Errorf("self-check 40h ago: status %d, want stRec (slipped)", rows[1].status)
	}
	if rows[1].value == "" {
		t.Error("a slipped row must still report how long ago the last pass ran")
	}
}

// writeMarker plants one hq-feed marker file for the maintenance rows to read.
func writeMarker(t *testing.T, home, name, body string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "gtmux", "hq-feed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The consumption row is the observability half of the wake watermark: perception is
// silent in BOTH directions, so an HQ that stopped consuming looks exactly like a fleet
// where nothing happened. Before this row the only detector was the commander noticing
// that a finished job went unremarked.
func TestHQConsumptionCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const now = 10_000_000

	if got := hqConsumptionCheck(now); got.status != stInfo {
		t.Errorf("no watermark yet: status %d, want a neutral note", got.status)
	}

	// Caught up → ✓.
	hqwake.Consume(0)
	if got := hqConsumptionCheck(now); got.status != stOK {
		t.Errorf("caught up: status %d, want stOK", got.status)
	}

	// Behind, and standing far too long → ⚠, naming both the count and the age.
	for i := 0; i < 3; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Pane: "%16"})
	}
	if err := os.WriteFile(filepath.Join(home, ".local", "share", "gtmux", "hqwake", "unread-state"),
		[]byte("0 9996400 0"), 0o644); err != nil { // standing an hour
		t.Fatal(err)
	}
	got := hqConsumptionCheck(now)
	if got.status != stRec {
		t.Errorf("an hour behind: status %d, want stRec", got.status)
	}
	if !strings.Contains(got.value, "3") {
		t.Errorf("value %q should report how far behind HQ is", got.value)
	}
}

// rowConfig: absent → neutral defaults, valid JSON → OK, malformed → recommended.
func TestRowConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if r := rowConfig(); r.status != stInfo {
		t.Errorf("missing config → stInfo, got %d", r.status)
	}
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := rowConfig(); r.status != stOK {
		t.Errorf("valid config → stOK, got %d", r.status)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{bad`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := rowConfig(); r.status != stRec {
		t.Errorf("invalid config → stRec, got %d", r.status)
	}
}

// dirCountSize counts files + bytes across a tree; absent dir → 0,0.
func TestDirCountSize(t *testing.T) {
	dir := t.TempDir()
	if n, sz := dirCountSize(filepath.Join(dir, "nope")); n != 0 || sz != 0 {
		t.Errorf("absent dir → 0,0; got %d,%d", n, sz)
	}
	_ = os.WriteFile(filepath.Join(dir, "a"), []byte("hello"), 0o644)  // 5
	_ = os.WriteFile(filepath.Join(dir, "b"), []byte("world!"), 0o644) // 6
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "c"), []byte("x"), 0o644) // 1
	if n, sz := dirCountSize(dir); n != 3 || sz != 12 {
		t.Errorf("dirCountSize = %d files, %d bytes; want 3, 12", n, sz)
	}
}

// terminalInstalled finds a .app bundle in ~/Applications; absent → false. Uses a
// fake bundle name so a real terminal in /Applications (which a temp HOME can't
// isolate) doesn't make the negative case flaky.
// forceEnglish pins the output language to "en" for note-wording assertions,
// restoring the prior language on cleanup.
func forceEnglish(t *testing.T) {
	t.Helper()
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })
}

// TestRowTerminalHonestPerHost pins the env-doctor spec's "Terminal landscape"
// honesty tiers on the HOST row: a fully-driven host (Ghostty/iTerm2) claims
// focus/restore/new; the best-effort Warp host states its limits (restore/new
// work, focus is best-effort) and never claims full support; an undriven host
// says agents still work but focus/restore don't.
func TestRowTerminalHonestPerHost(t *testing.T) {
	forceEnglish(t)

	t.Setenv("GTMUX_TERMINAL", "ghostty")
	if d := rowTerminal(); d.status != stOK || !strings.Contains(d.note, "focus / restore / new supported") {
		t.Errorf("ghostty host: got status=%d note=%q", d.status, d.note)
	}

	t.Setenv("GTMUX_TERMINAL", "warp")
	d := rowTerminal()
	if d.status != stOK {
		t.Errorf("warp host: status=%d, want stOK", d.status)
	}
	for _, want := range []string{"restore / new supported", "best-effort"} {
		if !strings.Contains(d.note, want) {
			t.Errorf("warp host note %q lacks %q", d.note, want)
		}
	}
	if strings.Contains(d.note, "focus / restore / new supported") {
		t.Errorf("warp host must not claim full support: %q", d.note)
	}

	t.Setenv("GTMUX_TERMINAL", "alacritty")
	if d := rowTerminal(); d.status != stRec || !strings.Contains(d.note, "no driver") {
		t.Errorf("undriven host: got status=%d note=%q", d.status, d.note)
	}
}

// TestOtherTerminalsWarpBestEffort pins the OTHER-terminals row's third tier:
// an installed Warp is marked "(best-effort)" — not "(supported)", which would
// overclaim, and not "(sensed)", which would hide the driver it has.
func TestOtherTerminalsWarpBestEffort(t *testing.T) {
	forceEnglish(t)
	t.Setenv("GTMUX_TERMINAL", "ghostty") // host ≠ warp, so Warp lists among the others
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "Applications", "Warp.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := rowOtherTerminals()
	if !strings.Contains(d.note, "Warp (best-effort)") {
		t.Errorf("other-terminals row %q must mark Warp best-effort", d.note)
	}
}

func TestTerminalInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	const fake = "ZzGtmuxDoctorTest.app" // won't exist in /Applications
	if terminalInstalled(fake) {
		t.Error("no such app → false")
	}
	if err := os.MkdirAll(filepath.Join(home, "Applications", fake), 0o755); err != nil {
		t.Fatal(err)
	}
	if !terminalInstalled(fake) {
		t.Error("bundle present in ~/Applications → true")
	}
}

// The write probe changes nothing that lasts: a store whose directory does not exist yet
// is checked through its nearest existing parent, and no directory is created; it used to
// leave two empty levels behind (%12, 2026-10-06). The probe file never outlives the check.
func TestStoreWriteProbeChangesNothing(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "a", "b", "events.jsonl")
	if err := probeStoreWrite(missing); err != nil {
		t.Fatalf("missing dir under a writable parent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !os.IsNotExist(err) {
		t.Fatalf("the probe created %s (err %v)", filepath.Join(root, "a"), err)
	}

	dir := filepath.Join(root, "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "day.jsonl")
	if err := os.WriteFile(file, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := probeStoreWrite(file); err != nil {
		t.Fatalf("existing file: %v", err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("the probe left files behind: %v", ents)
	}
	if b, _ := os.ReadFile(file); string(b) != "x\n" {
		t.Fatalf("the probe changed the file: %q", b)
	}

	if os.Getuid() != 0 {
		ro := filepath.Join(root, "ro")
		if err := os.MkdirAll(ro, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
		if err := probeStoreWrite(filepath.Join(ro, "x", "y.jsonl")); err == nil {
			t.Error("a store under a read-only parent read as writable")
		}
	}
}

// The locale fix sets LANG only where LANG decides: with LC_ALL or LC_CTYPE set it changes
// nothing and names the variable to change, since it outranks LANG and the user set it
// (%12, 2026-10-06). It also leaves a UTF-8 server, and a server it could not ask, alone.
// None of these cases reaches tmux or writes the config.
func TestStepLocaleLeavesWhatItCannotFix(t *testing.T) {
	t.Setenv("GTMUX_LANG", "en")
	for _, tc := range []struct {
		name  string
		env   map[string]string
		known bool
	}{
		{"LC_ALL outranks LANG", map[string]string{"LC_ALL": "C", "LANG": "C"}, true},
		{"LC_CTYPE outranks LANG", map[string]string{"LC_CTYPE": "C"}, true},
		{"already UTF-8", map[string]string{"LANG": "en_US.UTF-8"}, true},
		{"server could not be asked", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withLocaleEnv(t, tc.env, true, tc.known)
			conf := filepath.Join(t.TempDir(), "tmux.conf")
			s := &fixState{yes: true, confPath: conf}
			if n := s.stepLocale(); n != 0 {
				t.Fatalf("stepLocale applied %d change(s)", n)
			}
			if _, err := os.Stat(conf); !os.IsNotExist(err) {
				t.Fatalf("the config was written (err %v)", err)
			}
		})
	}
}
