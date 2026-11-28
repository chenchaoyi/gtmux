package prompt

import "testing"

func TestCHH008_BootBannerDisplayLabelAndAllStartupChoosers(t *testing.T) {
	codexBoot := "• Starting MCP servers (1/4): github (0s • esc to interrupt)\n› "
	if got := BootBannerLine(codexBoot, "Codex"); got == "" {
		t.Fatal("BootBannerLine missed Codex MCP boot banner when given display label 'Codex'")
	}
	if IsComposerReady(codexBoot, "Codex") {
		t.Fatal("IsComposerReady returned true while Codex MCP boot banner was active")
	}

	// Display labels resolve per-agent startup gates while keeping Codex/Antigravity
	// trust gates actionable in WaitingOptions; Claude's default gate and all
	// reopened-session pickers are excluded from WaitingOptions.
	codexTrust := "Do you trust the contents of this directory?\n› 1. Yes, continue\n  2. No, quit\nPress enter to confirm"
	if !IsStartupGate(codexTrust, "Codex") || len(WaitingOptions(codexTrust)) != 2 {
		t.Fatal("Codex trust gate must match IsStartupGate('Codex') and surface 2 WaitingOptions")
	}
	agyTrust := "Do you trust the contents of this project?\n> 1. Trust project\n  2. Exit\n↑/↓ Navigate · Enter to select"
	if !IsStartupGate(agyTrust, "Antigravity") || len(WaitingOptions(agyTrust)) != 2 {
		t.Fatal("Antigravity trust gate must match IsStartupGate('Antigravity') and surface 2 WaitingOptions")
	}

	excluded := []struct {
		name string
		text string
	}{
		{
			name: "claude-default-trust",
			text: "Do you trust the files in this folder?\n❯ 1. Yes, proceed\n  2. No, exit\nEnter to confirm",
		},
		{
			name: "resume-picker",
			text: "How would you like to resume?\n❯ 1. Resume from summary\n  2. Resume full session\nEnter to confirm",
		},
	}
	for _, tc := range excluded {
		t.Run(tc.name, func(t *testing.T) {
			if opts := WaitingOptions(tc.text); opts != nil {
				t.Fatalf("WaitingOptions(%s) = %+v, want nil for excluded startup chooser", tc.name, opts)
			}
		})
	}
}
