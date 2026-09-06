package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Kimi's hooks go into a file gtmux does NOT own — the same config.toml that holds the
// user's providers, models and API keys. Every test here is about that: what gtmux
// writes, and what it must never disturb.

const userConfig = `# my kimi setup
default_model = "k2"

[providers.moonshot]
api_key = "sk-secret"
base_url = "https://api.moonshot.cn/v1"

[[hooks]]
event = "Stop"
command = "say done"
timeout = 5
`

func kimiHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", dir)
	return filepath.Join(dir, "config.toml")
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestKimiInstallLeavesTheUsersConfigIntact(t *testing.T) {
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "/usr/local/bin/gtmux", true); err != nil {
		t.Fatalf("install: %v", err)
	}
	got := read(t, path)

	// Every line the user wrote is still there, in order, untouched.
	if !strings.HasPrefix(got, userConfig) {
		t.Errorf("the user's config is no longer a prefix of the file:\n%s", got)
	}
	// Including their own hook, which is not ours to manage.
	if !strings.Contains(got, `command = "say done"`) {
		t.Error("a foreign [[hooks]] entry was lost")
	}
	if !strings.Contains(got, `api_key = "sk-secret"`) {
		t.Error("the provider table was lost")
	}
	if !strings.Contains(got, `command = "/usr/local/bin/gtmux hook --agent kimi UserPromptSubmit"`) {
		t.Error("gtmux's own entry was not written")
	}
}

func TestKimiInstallIsIdempotent(t *testing.T) {
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := updateKimiHooks(path, "/usr/local/bin/gtmux", true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	got := read(t, path)
	if n := strings.Count(got, kimiBlockBegin); n != 1 {
		t.Errorf("after 3 installs the file has %d managed blocks, want 1", n)
	}
	// The full quoted command, not a prefix: "…kimi Stop" also matches
	// "…kimi StopFailure".
	if n := strings.Count(got, `hook --agent kimi Stop"`); n != 1 {
		t.Errorf("Stop is registered %d times, want 1", n)
	}
}

func TestKimiInstallAfterTheBinaryMoves(t *testing.T) {
	// A stale entry pointing at a gtmux that is no longer there is a hook that fails
	// on every event, which Kimi swallows (fail-open) — silence, not an error.
	path := kimiHome(t)
	if err := updateKimiHooks(path, "/old/path/gtmux", true); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "/new/path/gtmux", true); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if strings.Contains(got, "/old/path/gtmux") {
		t.Error("an entry still points at the old binary path")
	}
	if !strings.Contains(got, "/new/path/gtmux") {
		t.Error("the new binary path was not written")
	}
}

func TestKimiUninstallRemovesOnlyOurBlock(t *testing.T) {
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "/usr/local/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "", false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	got := read(t, path)
	if strings.Contains(got, "gtmux") {
		t.Errorf("uninstall left gtmux content behind:\n%s", got)
	}
	if strings.TrimSpace(got) != strings.TrimSpace(userConfig) {
		t.Errorf("uninstall did not restore the user's config.\n got:\n%s\nwant:\n%s", got, userConfig)
	}
}

func TestKimiUninstallTakesAFileThatWasOnlyOurs(t *testing.T) {
	// gtmux created config.toml because the user had none. Leaving an empty one
	// behind would be litter, and Kimi treats a missing config as "no config".
	path := kimiHome(t)
	if err := updateKimiHooks(path, "/usr/local/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config.toml survived an uninstall that emptied it: %v", err)
	}
}

func TestKimiUninstallOnAConfigWeNeverTouched(t *testing.T) {
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "", false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if got := read(t, path); got != userConfig {
		t.Errorf("an uninstall with nothing of ours installed rewrote the file:\n%s", got)
	}
}

func TestKimiUninstallOnAMissingConfig(t *testing.T) {
	path := kimiHome(t)
	if err := updateKimiHooks(path, "", false); err != nil {
		t.Errorf("uninstall with no config at all should be a no-op, got %v", err)
	}
}

func TestKimiBlockSurvivesAPathWithQuotes(t *testing.T) {
	// An unescaped quote or backslash in the command string would not corrupt one
	// hook — it would make the WHOLE config fail to load, taking the user's provider
	// credentials down with it.
	path := kimiHome(t)
	weird := `/Users/a "b"\c/gtmux`
	if err := updateKimiHooks(path, weird, true); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if strings.Contains(got, `"/Users/a "b"`) {
		t.Errorf("the path was written unescaped:\n%s", got)
	}
	if !strings.Contains(got, `\"b\"`) || !strings.Contains(got, `\\c`) {
		t.Errorf("expected TOML escaping of quote and backslash:\n%s", got)
	}
}

func TestKimiAppendsAfterTheLastTable(t *testing.T) {
	// A `[[hooks]]` block appended inside another table's scope would belong to that
	// table. It cannot happen — a table header ends the previous scope — but the
	// block must still come after every line of the user's file, never spliced in.
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if strings.Index(got, kimiBlockBegin) < strings.Index(got, "sk-secret") {
		t.Error("the managed block was written before the end of the user's config")
	}
}

func TestKimiBindingsCoverTheEventsGtmuxNeeds(t *testing.T) {
	// The tier depends on these three arriving. A binding dropped by an edit is a
	// silent demotion — no waiting, no done, no receipt-verified send.
	need := map[string]bool{"UserPromptSubmit": false, "Stop": false, "PermissionRequest": false}
	for _, b := range kimiBindings {
		if _, ok := need[b.event]; ok {
			need[b.event] = true
		}
		if b.timeoutSec < 1 || b.timeoutSec > 600 {
			t.Errorf("%s timeout %ds is outside Kimi's accepted 1–600s range", b.event, b.timeoutSec)
		}
	}
	for evt, seen := range need {
		if !seen {
			t.Errorf("no binding for %s — the event layer would be incomplete", evt)
		}
	}
}

func TestKimiApprovalIsSeparateFromPreToolUse(t *testing.T) {
	// Kimi raises a dedicated PermissionRequest, so PreToolUse must stay telemetry.
	// Mapping PreToolUse to the approval token would flag "needs you" on every single
	// tool call the agent makes.
	for _, b := range kimiBindings {
		if b.event == "PreToolUse" && b.token == "PermissionRequest" {
			t.Error("PreToolUse is mapped to PermissionRequest — every tool call would read as needing you")
		}
	}
}

func TestStripKimiBlockLeavesAnUnrelatedSentinelAlone(t *testing.T) {
	text := "a = 1\n# >>> someone else's block\nb = 2\n# <<< theirs\n"
	got, found := stripKimiBlock(text)
	if found {
		t.Error("claimed to find a gtmux block in a file that has none")
	}
	if got != text {
		t.Errorf("rewrote a file it should not have touched:\n%s", got)
	}
}

// TestKimiRealBinaryAcceptsWhatWeWrite runs Kimi's OWN validator over the block.
//
// Every other test here checks what gtmux intended to write. This one checks that Kimi
// agrees, which is the only claim that matters: `[[hooks]]` accepts exactly four fields
// and an unknown one makes the WHOLE config fail to load, taking the user's provider
// credentials with it. Measured against kimi 0.41.0 — adding an `owner = "gtmux"` field
// produced `hooks[11]: Unrecognized key: "owner"`, which is why ownership is carried by
// sentinel COMMENTS instead of a field.
//
// Skipped where Kimi is not installed, so CI stays green without it; it is a real-machine
// check, not a unit test.
func TestKimiRealBinaryAcceptsWhatWeWrite(t *testing.T) {
	bin := filepath.Join(os.Getenv("HOME"), ".kimi-code", "bin", "kimi")
	if _, err := os.Stat(bin); err != nil {
		if p, lerr := exec.LookPath("kimi"); lerr == nil {
			bin = p
		} else {
			t.Skip("kimi is not installed")
		}
	}
	path := kimiHome(t)
	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateKimiHooks(path, "/usr/local/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	// `kimi doctor` exits 0 even when it reports a problem, so the VERDICT is read
	// from its output, not its status.
	out, _ := exec.Command(bin, "doctor").CombinedOutput()
	if strings.Contains(string(out), "ERROR") || !strings.Contains(string(out), "valid") {
		t.Errorf("kimi rejected the config gtmux wrote:\n%s\n--- config ---\n%s", out, read(t, path))
	}
}

// --- doctor -----------------------------------------------------------------

func TestKimiDoctorRowKnowsInstalledFromComplete(t *testing.T) {
	// Codex taught this one: "the block is there" and "the block carries the events
	// gtmux now uses" are different claims, and a file written by an older gtmux read
	// a plain green ✓ while missing every event added since.
	path := kimiHome(t)

	if got := missingKimiHookEvents(); got != nil {
		t.Errorf("with no config at all, missingKimiHookEvents() = %v, want nil (not installed)", got)
	}

	if err := os.WriteFile(path, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := missingKimiHookEvents(); got != nil {
		t.Errorf("with a foreign-only config, got %v, want nil (not installed)", got)
	}

	if err := updateKimiHooks(path, "/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	got := missingKimiHookEvents()
	if got == nil || len(got) != 0 {
		t.Errorf("a fresh install reports %v, want an empty slice (installed, complete)", got)
	}
}

func TestKimiDoctorSpotsABlockThatPredatesAnEvent(t *testing.T) {
	path := kimiHome(t)
	if err := updateKimiHooks(path, "/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	// An older gtmux that never registered StopFailure.
	text := read(t, path)
	text = strings.Replace(text, `command = "/bin/gtmux hook --agent kimi StopFailure"`, `command = "/bin/gtmux hook --agent kimi Nope"`, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got := missingKimiHookEvents()
	if len(got) != 1 || got[0] != "StopFailure" {
		t.Errorf("missingKimiHookEvents() = %v, want [StopFailure]", got)
	}
}

func TestKimiDoctorIsNotFooledByATokenPrefix(t *testing.T) {
	// "…kimi Stop" is a prefix of "…kimi StopFailure": matching without the closing
	// quote would report a MISSING Stop as present.
	path := kimiHome(t)
	if err := updateKimiHooks(path, "/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(read(t, path), `command = "/bin/gtmux hook --agent kimi Stop"`, `command = "/bin/gtmux hook --agent kimi Gone"`, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got := missingKimiHookEvents()
	if len(got) != 1 || got[0] != "Stop" {
		t.Errorf("missingKimiHookEvents() = %v, want [Stop] — StopFailure masked a missing Stop", got)
	}
}

func TestKimiDoctorRowReadsTheThreeStates(t *testing.T) {
	// The row is what a user sees, so assert on the row and not only on its helper.
	path := kimiHome(t)

	if got := rowKimiHook(); got.status != stRec {
		t.Errorf("with nothing installed the row is %v, want a recommendation", got.status)
	}
	if err := updateKimiHooks(path, "/bin/gtmux", true); err != nil {
		t.Fatal(err)
	}
	if got := rowKimiHook(); got.status != stOK {
		t.Errorf("with a complete install the row is %v, want OK", got.status)
	}
	text := strings.Replace(read(t, path), `hook --agent kimi PreCompact"`, `hook --agent kimi X"`, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got := rowKimiHook()
	if got.status != stRec || !strings.Contains(got.note, "PreCompact") {
		t.Errorf("an incomplete block reads %v / %q — it should name the missing event", got.status, got.note)
	}
}
