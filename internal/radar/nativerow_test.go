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

// The agent's saved conversation name is the row title; cwd is only the fallback.
// This checks the native record -> transcript index -> radar task boundary rather
// than just testing the index reader in isolation.
func TestNativeCodexPanesCarrySavedSessionTitles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	index := filepath.Join(codexHome, "session_index.jsonl")
	if err := os.WriteFile(index, []byte(
		"{\"id\":\"named\",\"thread_name\":\"Investigate SpringBoard crash\"}\n"+
			"{\"id\":\"other\",\"thread_name\":\"Another session\"}\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	for _, id := range []string{"named", "untitled"} {
		if err := native.Save(native.Record{
			SessionID: id, Agent: "codex", State: "working", UpdatedAt: now,
			Cwd: t.TempDir(), PID: os.Getpid(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	panes := nativePanes(nil, nil, now)
	if len(panes) != 2 {
		t.Fatalf("nativePanes = %d rows, want 2", len(panes))
	}
	byID := make(map[string]Pane, len(panes))
	for _, p := range panes {
		byID[p.sessionID] = p
	}
	if got := byID["named"].Task; got != "Investigate SpringBoard crash" {
		t.Errorf("saved title = %q, want Codex thread name", got)
	}
	if got := byID["untitled"].Task; got != "" {
		t.Errorf("untitled task = %q, want empty so clients use project fallback", got)
	}
	if byID["named"].source != "native" || byID["named"].Loc != "" || byID["named"].Status != "working" {
		t.Errorf("adding a title changed native session identity/state: %+v", byID["named"])
	}
}

func TestNativeCodexClientControlsMoveEligibility(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	dir := filepath.Join(codexHome, "sessions", "2026", "09", "29")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	for _, tc := range []struct{ id, originator string }{
		{"desktop-native", "codex_work_desktop"},
		{"desktop-display-name-native", "Codex Desktop"},
		{"terminal-native", "codex-tui"},
	} {
		data := `{"type":"session_meta","payload":{"id":"` + tc.id + `","originator":"` + tc.originator + `","source":"vscode"}}` + "\n" +
			`{"timestamp":"2026-09-29T00:00:01Z","type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-29T00-00-00-"+tc.id+".jsonl"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := native.Save(native.Record{SessionID: tc.id, Agent: "codex", State: "idle", UpdatedAt: now, PID: os.Getpid()}); err != nil {
			t.Fatal(err)
		}
	}
	panes := nativePanes(nil, nil, now)
	if len(panes) != 3 {
		t.Fatalf("native client provenance = %+v", panes)
	}
	byID := map[string]Pane{}
	for _, p := range panes {
		byID[p.sessionID] = p
	}
	for _, id := range []string{"desktop-native", "desktop-display-name-native"} {
		if p := byID[id]; p.source != "native" || p.client != "chatgpt_desktop" || p.adoptable {
			t.Errorf("desktop move should be hidden: %+v", p)
		}
	}
	if p := byID["terminal-native"]; p.source != "native" || p.client != "terminal" || !p.adoptable {
		t.Errorf("terminal move should remain available: %+v", p)
	}
}

func TestNativeCodexWorkingStateReconcilesWithRollout(t *testing.T) {
	for _, tc := range []struct {
		name, boundary string
		boundaryOffset int64
		want           string
	}{
		{"completed after hook", "task_complete", 10, "idle"},
		{"aborted after hook", "turn_aborted", 10, "idle"},
		{"new turn after hook", "task_started", 10, "working"},
		{"older completion", "task_complete", -10, "working"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			codexHome := t.TempDir()
			t.Setenv("CODEX_HOME", codexHome)
			const sessionID = "native-codex"
			now := time.Now().Unix()
			updatedAt := now - 20
			if err := native.Save(native.Record{
				SessionID: sessionID, Agent: "codex", State: "working",
				UpdatedAt: updatedAt, PID: os.Getpid(),
			}); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(codexHome, "sessions", "2026", "09", "28")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			line := `{"timestamp":"` + time.Unix(updatedAt+tc.boundaryOffset, 0).UTC().Format(time.RFC3339Nano) +
				`","type":"event_msg","payload":{"type":"` + tc.boundary + `"}}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-28T00-00-00-"+sessionID+".jsonl"), []byte(line), 0o600); err != nil {
				t.Fatal(err)
			}
			panes := nativePanes(nil, nil, now)
			if len(panes) != 1 || panes[0].Status != tc.want {
				t.Fatalf("native Codex status = %+v, want %s", panes, tc.want)
			}
			if tc.want == "idle" && !panes[0].adoptable {
				t.Error("completed native Codex session should be movable")
			}
		})
	}
}

func TestNativeCodexResumedRolloutCompletesAfterHook(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	const sid = "desktop-session"
	now := time.Date(2026, 9, 29, 7, 40, 0, 0, time.UTC).Unix()
	hookAt := now - 3600
	// No pid: this test is about rollout state, and its dates are fixed to match the
	// rollouts. A record dated 2026-09-29 naming THIS process (started later) is a pid the
	// liveness check rightly reads as reused; the on-disk conversation is the evidence.
	if err := native.Save(native.Record{SessionID: sid, Agent: "codex", State: "working", UpdatedAt: hookAt}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(codexHome, "sessions", "2026", "09", "29")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, lines ...string) {
		t.Helper()
		body := `{"type":"session_meta","payload":{"id":"` + sid + `","originator":"codex_work_desktop"}}` + "\n" + strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("rollout-2026-09-29T06-00-00-"+sid+".jsonl",
		`{"timestamp":"2026-09-29T06:00:00Z","type":"event_msg","payload":{"type":"task_started"}}`)
	write("rollout-2026-09-29T07-00-00-"+sid+"_instance.jsonl",
		`{"timestamp":"2026-09-29T07:30:59Z","type":"event_msg","payload":{"type":"task_complete"}}`)
	panes := nativePanes(nil, nil, now)
	if len(panes) != 1 || panes[0].Status != "idle" || panes[0].Since != now-541 || panes[0].client != "chatgpt_desktop" || panes[0].adoptable {
		t.Fatalf("resumed desktop Codex = %+v", panes)
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

// A native session whose turn died on an API error (the hook leaves its record idle on a
// StopFailure) is marked errored, as a tmux row is, so it does not read as a finish.
func TestNativeRowEndedOnAnErrorIsErrored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Now().Unix()
	writeTranscript(t, home, "crashed", "API Error: Connection lost mid-response.", true)
	writeTranscript(t, home, "finished", "All done.", false)
	for _, sid := range []string{"crashed", "finished"} {
		if err := native.Save(native.Record{SessionID: sid, Agent: "claude", State: "idle", UpdatedAt: now, PID: os.Getpid()}); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]Pane{}
	for _, p := range nativePanes(nil, nil, now) {
		got[p.sessionID] = p
	}
	if p := got["crashed"]; p.Status != "idle" || !p.Errored || !strings.Contains(p.ErrorText, "Connection lost") {
		t.Errorf("crashed = %+v, want idle and errored with the error text", p)
	}
	if p := got["finished"]; p.Errored {
		t.Errorf("finished = %+v, want not errored", p)
	}
}
