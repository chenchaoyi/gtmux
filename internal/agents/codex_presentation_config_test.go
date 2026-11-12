package agents_test

import (
	"github.com/chenchaoyi/gtmux/internal/agents"
	"github.com/chenchaoyi/gtmux/internal/codexbridge"
	"testing"
)

func TestSID001PresentationConfigKeepsManagedIdentity(t *testing.T) {
	for _, args := range [][]string{
		{"--no-alt-screen", "--approve-for-me", "-c", `tui.notifications=["approval-requested","plan-mode-prompt"]`},
		{"--config=tui.notifications=false", "resume", "root"},
		{"--config", `tui.notifications=[]`},
		{"-c", `tui.notifications=true`},
	} {
		if !agents.CodexBridgeEligible(args) {
			t.Errorf("display-only override bypassed managed identity: %q", args)
		}
	}
	for _, args := range [][]string{
		{"-c", `model_provider="custom"`}, {"--config", `codex_home="/other"`},
		{"-c", `approval_policy="never"`}, {"-c", `sandbox_mode="danger-full-access"`},
		{"-c", `tui.notifications=["x"]` + "\nmodel_provider=\"custom\""},
		{"-c", `tui.notifications={enable=true}`}, {"-c", `tui.notifications=null`},
		{"-c", `tui.notifications=[null]`}, {"-c", `tui.notifications=[true]`}, {"-c", `tui.notifications=[1]`}, {"-c"}, {"--config="},
		{"--remote", "unix:///synthetic", "-c", `tui.notifications=false`},
		{"--profile", "custom", "-c", `tui.notifications=false`},
	} {
		if agents.CodexBridgeEligible(args) {
			t.Errorf("connection/permission/unknown override admitted: %q", args)
		}
	}
}

func TestSID001HelpStillRecognizesCommandsAfterDisplayConfig(t *testing.T) {
	help := "Commands:\n  exec Run\n  resume Resume\n  fork Fork\n  future-cmd Future\n\nOptions:\n"
	if codexbridge.HelpAdmits([]string{"-c", `tui.notifications=false`, "future-cmd"}, help) {
		t.Fatal("display value hid the native subcommand")
	}
	if !codexbridge.HelpAdmits([]string{"--config=tui.notifications=false", "resume", "root"}, help) {
		t.Fatal("interactive resume with display override refused")
	}
}
