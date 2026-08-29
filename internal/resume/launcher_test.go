package resume

import "testing"

// An agent is not always launched by its own name. A Codex session started through an
// internal wrapper carries that wrapper's configuration and credentials; resuming it as
// a bare `codex` brings the conversation back without any of them.
func TestResumeUsesTheRecordedLauncher(t *testing.T) {
	cmd, ok := Command(Record{Agent: "codex", SessionID: "01a04111", Launcher: "opencrab"})
	if !ok {
		t.Fatal("a wrapper-launched record must still be resumable")
	}
	if !contains(cmd, "opencrab resume '01a04111'") {
		t.Errorf("resumed as %q — it must run through the wrapper", cmd)
	}
	if contains(cmd, "codex resume") {
		t.Errorf("still resuming by the agent's own name: %q", cmd)
	}
}

// Only the executable is substituted. A wrapper forwards its arguments to the agent; it
// does not redefine them, so the resume flags stay the registry's.
func TestLauncherKeepsTheAgentsOwnFlags(t *testing.T) {
	cmd, _ := Command(Record{Agent: "claude", SessionID: "abc", Launcher: "myclaude"})
	if !contains(cmd, "myclaude --resume 'abc'") {
		t.Errorf("got %q, want the agent's own --resume flag through the wrapper", cmd)
	}
}

// A record written before launchers were recorded must resume exactly as well as it did.
func TestNoLauncherResumesAsBefore(t *testing.T) {
	cmd, ok := Command(Record{Agent: "codex", SessionID: "abc"})
	if !ok || !contains(cmd, "codex resume 'abc'") {
		t.Errorf("got %q (ok=%v), want the registry form unchanged", cmd, ok)
	}
}

// The cwd guard is what makes --resume find the conversation at all (agents file their
// transcript under the launch dir), so a launcher must not displace it.
func TestLauncherKeepsTheCwdGuard(t *testing.T) {
	cmd, _ := Command(Record{Agent: "codex", SessionID: "abc", Cwd: "/tmp/work", Launcher: "opencrab"})
	if !contains(cmd, "cd -- '/tmp/work'") || !contains(cmd, "opencrab resume") {
		t.Errorf("got %q, want the cwd guard AND the wrapper", cmd)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// restore does not use the stored record directly — it passes it through Resolve first
// (which relocates a Claude conversation to where it is actually filed). A field added
// to Record is only as good as its survival through that step.
func TestResolveKeepsTheLauncher(t *testing.T) {
	in := Record{Agent: "codex", SessionID: "abc", Launcher: "opencrab"}
	out, ok := Resolve(in)
	if !ok || out.Launcher != "opencrab" {
		t.Errorf("Resolve dropped the launcher: %+v (ok=%v)", out, ok)
	}
}
