package hook

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// A Claude pane's pane_current_command is its VERSION ("2.1.250"), not a command anyone
// can run. Recording that as the launcher would turn a resumable conversation into a
// command that fails — the mirror image of the bug this feature exists to fix. gtmux has
// been caught by this version-as-command shape before (#659).
func TestAVersionIsNeverALauncher(t *testing.T) {
	for _, v := range []string{"2.1.250", "0.146.0", "1.11.0"} {
		if !looksLikeVersion(v) {
			t.Errorf("%q must be recognised as a version, not a command", v)
		}
	}
	for _, c := range []string{"opencrab", "codex", "bash", "my-agent", "npx"} {
		if looksLikeVersion(c) {
			t.Errorf("%q is a command name, not a version", c)
		}
	}
}

// The rules have to be APPLIED, not merely correct in isolation: a first version of this
// test checked only the version helper, and removing the guard from the decision left it
// green.
func TestLauncherFromDecides(t *testing.T) {
	cases := []struct{ cmd, agent, want, why string }{
		{"opencrab", "codex", "opencrab", "a wrapper is exactly what must be recorded"},
		{"/usr/local/bin/opencrab", "codex", "opencrab", "a path is recorded by its name"},
		{"codex", "codex", "", "the agent under its own name is the registry form already"},
		{"claude", "claude", "", "same for claude"},
		{"bash", "codex", "", "a shell is the pane's own process, not a launcher"},
		{"-bash", "codex", "", "a login shell too"},
		{"2.1.250", "claude", "", "a Claude pane reports its VERSION — not a runnable command"},
		{"", "codex", "", "nothing to go on"},
	}
	for _, c := range cases {
		if got := launcherFrom(c.cmd, c.agent); got != c.want {
			t.Errorf("launcherFrom(%q, %q) = %q, want %q — %s", c.cmd, c.agent, got, c.want, c.why)
		}
	}
}

// A hook fires mid-turn too, and mid-turn the pane's foreground command can be a tool
// the agent is running. Reading it then would replace a known launcher with `node` —
// so every event but the identity ones must carry the last reading forward.
func TestMidTurnEventsCarryTheLauncherForward(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // state.Dir() keys off $HOME, and only $HOME
	const loc = "work:0.0"
	if err := resume.Save(loc, resume.Record{Agent: "codex", SessionID: "s1", Launcher: "opencrab"}); err != nil {
		t.Fatal(err)
	}
	// pane "" makes a fresh reading impossible — so what comes back is purely the
	// difference between carrying forward and re-reading.
	for _, ev := range []string{"Stop", "PostToolUse", "Notification", "Waiting"} {
		if got := launcherFor(loc, "", "codex", ev); got != "opencrab" {
			t.Errorf("%s returned %q — a mid-turn event must not overwrite a known launcher", ev, got)
		}
	}
	// The identity events are authoritative, including their empty answer: an agent
	// restarted under its own name must stop resuming through a wrapper it no longer uses.
	for _, ev := range []string{"UserPromptSubmit", "SessionStart", "Resumed"} {
		if got := launcherFor(loc, "", "codex", ev); got != "" {
			t.Errorf("%s returned %q — an identity event's reading is the record", ev, got)
		}
	}
}
