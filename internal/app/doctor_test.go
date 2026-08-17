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
			{stRec, "HQ session health", "ctx 20% · 24h", "over age 24h — `gtmux hq --rotate`"},
		}},
		{"x", []dcheck{{stMiss, "tmux", "", "install it"}}},
	}
	got := advisoryRemaining(secs)
	if len(got) != 2 {
		t.Fatalf("want 2 flagged rows, got %d: %+v", len(got), got)
	}
	if got[0].label != "HQ session health" || got[1].label != "tmux" {
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

// TestLocaleCharsetPrecedence checks POSIX precedence (LC_ALL > LC_CTYPE > LANG)
// and that rowLocale flags a non-UTF-8 / unset locale as recommended, OK otherwise.
func TestLocaleCharsetPrecedence(t *testing.T) {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		t.Setenv(k, "")
	}
	if got := localeCharset(); got != "" {
		t.Fatalf("all unset → %q, want empty", got)
	}
	if rowLocale().status != stRec {
		t.Error("unset locale → recommended")
	}

	t.Setenv("LANG", "en_US.UTF-8")
	if got := localeCharset(); got != "en_US.UTF-8" {
		t.Fatalf("LANG only → %q", got)
	}
	if rowLocale().status != stOK {
		t.Error("UTF-8 LANG → ok")
	}

	t.Setenv("LC_ALL", "C") // LC_ALL wins over a UTF-8 LANG
	if got := localeCharset(); got != "C" {
		t.Fatalf("LC_ALL precedence → %q", got)
	}
	if rowLocale().status != stRec {
		t.Error("LC_ALL=C overrides UTF-8 LANG → recommended")
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
	if len(rows) != 3 {
		t.Fatalf("want 3 maintenance rows (distill, self-check, promotions), got %d", len(rows))
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
	if rows[0].status != stOK {
		t.Errorf("distill 3d ago: status %d, want stOK", rows[0].status)
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
