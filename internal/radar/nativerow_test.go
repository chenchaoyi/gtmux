package radar

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/assets"
	"github.com/chenchaoyi/gtmux/internal/native"
)

// A native record's sensed terminal ("Warp", "Ghostty", …) must reach the radar
// row's `terminal` field — it's how the surfaces label an out-of-tmux agent's
// home ("Elsewhere · Warp"). Before this, nativePanes dropped it: every native
// row shipped with no terminal name at all (verified live with a Warp-hosted
// hook, 2026-08-08).
func TestNativePanesCarryTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	if err := native.Save(native.Record{
		SessionID: "warp-native-1", Agent: "claude", State: "working",
		UpdatedAt: now - 30, Terminal: "Warp",
		PID: os.Getpid(), // positive-evidence gate: a live process keeps the row shown
	}); err != nil {
		t.Fatal(err)
	}
	panes := nativePanes(nil, nil, now)
	if len(panes) != 1 {
		t.Fatalf("nativePanes = %d rows, want 1", len(panes))
	}
	if panes[0].terminal != "Warp" {
		t.Fatalf("native row terminal = %q, want Warp", panes[0].terminal)
	}
	if panes[0].source != "native" {
		t.Fatalf("native row source = %q, want native", panes[0].source)
	}
}

// A native row must carry the same icon hint its tmux twin gets. The mobile avatar fetches
// /api/icon only when the row's `icon` is non-empty, so an empty hint is not a cosmetic
// nicety — it is the difference between the agent's real mark and a neutral monogram.
//
// Codex is the case that broke (seen on a phone, 2026-08-09: two "Cx" boxes under
// ELSEWHERE while Claude rows two lines up showed their mark): it ships no desktop app, so
// its profile carries no icon PATH and it depends on IconFor's `builtin:<key>` fallback to
// the committed PNG. nativePanes read `p.Icon` raw and skipped that fallback entirely.
func TestNativePanesCarryIcon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	if err := native.Save(native.Record{
		SessionID: "codex-native-1", Agent: "codex", State: "working", UpdatedAt: now - 30,
		PID: os.Getpid(), // positive-evidence gate: a live process keeps the row shown
	}); err != nil {
		t.Fatal(err)
	}
	panes := nativePanes(nil, LoadProfiles(), now)
	if len(panes) != 1 {
		t.Fatalf("nativePanes = %d rows, want 1", len(panes))
	}
	if panes[0].icon == "" {
		t.Fatal("native row has no icon hint — the phone will render the neutral monogram")
	}
	// The same agent through the tmux path resolves the identical hint; the two surfaces
	// must not disagree about who an agent is.
	if want := IconFor(panes[0].Agent, LoadProfiles()); panes[0].icon != want {
		t.Errorf("native icon = %q, want %q (what the tmux row gets)", panes[0].icon, want)
	}
}

// The icon hint must be a PATH a reader can OPEN, not a token a reader must be taught.
// The menu-bar app resolves the hint by opening it as a file, so the old `builtin:<key>`
// token fell through to the neutral monogram there while the phone and web (which only
// need a non-empty hint, then fetch the bytes from /api/icon) were fine — Codex had no
// icon on the menu bar at all.
func TestBuiltinIconHintIsAnOpenablePath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	hint := IconFor("Codex", LoadProfiles())
	if hint == "" {
		t.Fatal("Codex ships a committed icon — the hint must not be empty")
	}
	if strings.HasPrefix(hint, "builtin:") {
		t.Fatalf("the hint must be a path, not a token; got %q", hint)
	}
	b, err := os.ReadFile(hint)
	if err != nil {
		t.Fatalf("the hint must be openable: %v", err)
	}
	if !bytes.Equal(b, assets.AgentIcon("codex")) {
		t.Error("the materialized file must be the committed icon, byte for byte")
	}

	// Idempotent: a second call re-uses the file rather than rewriting it, because this
	// runs on every radar row of every poll.
	fi, err := os.Stat(hint)
	if err != nil {
		t.Fatal(err)
	}
	if again := IconFor("Codex", LoadProfiles()); again != hint {
		t.Errorf("the path must be stable across calls: %q then %q", hint, again)
	}
	fi2, err := os.Stat(hint)
	if err != nil {
		t.Fatal(err)
	}
	if !fi2.ModTime().Equal(fi.ModTime()) {
		t.Error("an unchanged icon must not be rewritten on every call")
	}
}

// A gtmux update can ship a NEW icon for an agent; a stale cached PNG must not outlive it.
func TestBuiltinIconRefreshesWhenTheBytesChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	p := BuiltinIconPath("codex")
	if p == "" {
		t.Fatal("codex ships a committed icon")
	}
	if err := os.WriteFile(p, []byte("a stale icon from an older release"), 0o644); err != nil {
		t.Fatal(err)
	}
	if again := BuiltinIconPath("codex"); again != p {
		t.Fatalf("path changed: %q", again)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, assets.AgentIcon("codex")) {
		t.Error("a stale cached icon must be refreshed from the committed bytes")
	}
}

// The ghost-Codex-rows regression (2026-08-09, misled the commander twice): an
// agent-internal helper call recorded as a native session shows a row with NOTHING
// behind it — no process (its PID can't be sensed), no on-disk conversation, nothing
// to focus, kill, or adopt — and a `working` one never resolves. The gate demands
// positive evidence: a native row is listed only when its record names a live process
// OR its session has an on-disk conversation. Both absent → the row is withheld.
func TestNativePanesHideRecordWithNoEvidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()

	// The ghost shape, verbatim from disk: no PID, no transcript, cwd "/".
	if err := native.Save(native.Record{
		SessionID: "ghost-1", Agent: "codex", State: "working", UpdatedAt: now - 60, Cwd: "/",
	}); err != nil {
		t.Fatal(err)
	}
	if panes := nativePanes(nil, nil, now); len(panes) != 0 {
		t.Fatalf("a record with no process and no conversation must be withheld, got %d rows", len(panes))
	}

	// A conversation on disk is evidence enough — a real session whose process gtmux
	// could not sense (Codex's detached hook path) must stay visible.
	dir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"timestamp":"` + time.Unix(now-120, 0).UTC().Format(time.RFC3339) + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "talked-1.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := native.Save(native.Record{
		SessionID: "talked-1", Agent: "claude", State: "idle", UpdatedAt: now - 60,
	}); err != nil {
		t.Fatal(err)
	}
	panes := nativePanes(nil, nil, now)
	if len(panes) != 1 || panes[0].sessionID != "talked-1" {
		t.Fatalf("a session with an on-disk conversation must stay listed, got %+v", panes)
	}
}

// An agent with no committed icon gets no hint — "" is the honest answer, and it is what
// tells a surface to draw its neutral monogram.
func TestBuiltinIconPathEmptyForUnknownAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := BuiltinIconPath("no-such-agent"); got != "" {
		t.Errorf("BuiltinIconPath(unknown) = %q, want empty", got)
	}
}
