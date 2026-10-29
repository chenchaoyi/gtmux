package codexbridge

import (
	"reflect"
	"testing"
)

func TestLaunchAdmissionPreservesNativeModes(t *testing.T) {
	for _, args := range [][]string{nil, {"--no-alt-screen"}, {"resume", "thread-id"}, {"fork", "--last"}, {"-C", "/synthetic space", "-m", "model", "fixed prompt"}} {
		if !Eligible(args) {
			t.Errorf("interactive launch excluded: %q", args)
		}
	}
	for _, args := range [][]string{{"--no-daemon"}, {"--remote", "unix:///synthetic"}, {"--profile", "work"}, {"-c", "model_provider=company"}, {"exec", "fixed"}, {"mcp-server"}, {"--future-option"}, {"--model"}} {
		if Eligible(args) {
			t.Errorf("native launch rewritten: %q", args)
		}
	}
}

func TestLocalBridgePreservesWorkingDirectoryAndExplicitOverride(t *testing.T) {
	for _, tc := range []struct{ args, want []string }{
		{nil, []string{"--cd", "/launch dir"}},
		{[]string{"--cd", "/explicit", "resume", "root"}, []string{"--cd", "/explicit", "resume", "root"}},
		{[]string{"-C", "/explicit", "prompt"}, []string{"-C", "/explicit", "prompt"}},
		{[]string{"resume", "root"}, []string{"--cd", "/launch dir", "resume", "root"}},
		{[]string{"--", "--cd"}, []string{"--cd", "/launch dir", "--", "--cd"}},
	} {
		if got := LocalArgs(tc.args, "/launch dir"); !reflect.DeepEqual(got, tc.want) {
			t.Fatal("working directory changed", got, tc.want)
		}
	}
}

func TestInstalledSubcommandsRemainNativeIncludingFutureCommands(t *testing.T) {
	help := `Usage: codex [OPTIONS]
Commands:
  exec       Run non-interactively
  resume     Resume a session
  fork       Fork a session
  plugin     Manage plugins
  update     Update Codex
  agents     Browse sessions
  future-cmd A command unknown when the adapter shipped
             [aliases: next-step, someday]

Options:
  --remote <ADDRESS>
  --no-daemon
`
	for _, command := range []string{"exec", "plugin", "update", "agents", "future-cmd", "next-step", "someday"} {
		if HelpAdmits([]string{"-C", "/workspace", command}, help) {
			t.Fatal("native command became an interactive bridge", command)
		}
	}
	for _, args := range [][]string{nil, {"resume", "root"}, {"fork", "--last"}, {"-C", "/workspace", "a normal prompt"}, {"--", "plugin"}} {
		if !HelpAdmits(args, help) {
			t.Fatal("interactive launch rejected", args)
		}
	}
	if HelpAdmits(nil, "unrecognized future help layout") {
		t.Fatal("unknown capabilities were inferred")
	}
}
