package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// A pane's resume record must name the conversation you can actually resume into.
// The bug (resume-record-ownership): Claude Code runs a slash command like `/usage` as
// its OWN session in the SAME pane, firing the same hooks — so it took the pane's record
// from the conversation running there. Measured on the reporter's machine: `gtmux:0.0`
// pointed at a 3.5 KB `/usage` stub while the real conversation's log was 72 MB, which
// blanked the chat view and would have made `restore` relaunch the stub after a reboot.

// plantLog writes a claude session log under a temp HOME. `real` decides whether it
// parses into a turn (a prompt AND an assistant reply) or is a command stub like the one
// `/usage` leaves behind.
func plantLog(t *testing.T, home, sid string, real bool) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "projects", "-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","sessionId":"` + sid + `","message":{"role":"user","content":[{"type":"text","text":"<command-name>/usage</command-name>"}]}}` + "\n"
	if real {
		body = `{"type":"user","sessionId":"` + sid + `","message":{"role":"user","content":[{"type":"text","text":"fix the parser"}]}}` + "\n" +
			`{"type":"assistant","sessionId":"` + sid + `","message":{"role":"assistant","content":[{"type":"text","text":"done — split verifyToken()"}]}}` + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, sid+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hookHome points HOME (agent logs) and the gtmux state dir at temp dirs.
func hookHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "share"))
	return home
}

func TestAnEmptyPaneIsClaimedFreely(t *testing.T) {
	home := hookHome(t)
	plantLog(t, home, "new", true)
	if !ownsPane("gtmux:0.0", "claude", "new") {
		t.Error("a pane with no record must accept the session reporting in it")
	}
}

func TestTheSameSessionAlwaysUpdatesItsOwnRecord(t *testing.T) {
	home := hookHome(t)
	const sid = "live"
	plantLog(t, home, sid, true)
	if err := resume.Save("gtmux:0.0", resume.Record{Agent: "claude", SessionID: sid}); err != nil {
		t.Fatal(err)
	}
	if !ownsPane("gtmux:0.0", "claude", sid) {
		t.Error("a session must always be able to refresh its OWN record")
	}
}

// The reported failure, in its own terms.
func TestASlashCommandStubCannotStealThePane(t *testing.T) {
	home := hookHome(t)
	plantLog(t, home, "real-work", true)
	plantLog(t, home, "usage-stub", false) // a `/usage` side session: no turns
	if err := resume.Save("gtmux:0.0", resume.Record{Agent: "claude", SessionID: "real-work"}); err != nil {
		t.Fatal(err)
	}

	if ownsPane("gtmux:0.0", "claude", "usage-stub") {
		t.Fatal("a command stub took the pane — restore would relaunch /usage instead of the work session")
	}
	// And the record is genuinely untouched.
	if rec, _ := resume.Load("gtmux:0.0"); rec.SessionID != "real-work" {
		t.Errorf("record = %q; want the real conversation retained", rec.SessionID)
	}
}

// A genuine handover (you quit the agent and started a fresh conversation in the pane)
// must still work — the guard is about stubs, not about newness.
func TestARealNewConversationTakesOverThePane(t *testing.T) {
	home := hookHome(t)
	plantLog(t, home, "old-work", true)
	plantLog(t, home, "new-work", true)
	if err := resume.Save("gtmux:0.0", resume.Record{Agent: "claude", SessionID: "old-work"}); err != nil {
		t.Fatal(err)
	}
	if !ownsPane("gtmux:0.0", "claude", "new-work") {
		t.Error("a real conversation must be able to take over a pane")
	}
}

// A record pointing at a conversation that no longer exists is worse than one pointing
// at a stub: it can never be resumed, and it would block the pane forever.
func TestADeadIncumbentDoesNotBlockThePane(t *testing.T) {
	home := hookHome(t)
	plantLog(t, home, "stub", false)
	if err := resume.Save("gtmux:0.0", resume.Record{Agent: "claude", SessionID: "vanished"}); err != nil {
		t.Fatal(err)
	}
	if !ownsPane("gtmux:0.0", "claude", "stub") {
		t.Error("an incumbent whose log is gone must not pin the pane")
	}
}

// The guard must never be reachable for an agent whose log layout we can't resolve in a
// way that silently freezes its records: an unresolvable INCUMBENT reads as dead.
func TestAnUnresolvableIncumbentReadsAsDead(t *testing.T) {
	hookHome(t)
	if logExists("some-unknown-agent", "abc") {
		t.Error("an agent whose log layout we can't resolve must not count as alive")
	}
	if logExists("claude", "") {
		t.Error("an empty session id is not a live log")
	}
}
