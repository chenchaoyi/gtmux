package prompt

import (
	"strings"
	"testing"
)

const folderAccessFixture = "Folder access\n/synthetic/repo/worktree\n\nTrust this folder? Codex can read, edit, and run files here, subject to your permission settings.\n\n› 1. Trust and continue\n  2. Back to Agent Command Center\n\nenter continue · esc back"

func TestWTR004CurrentFolderAccessBlocksReadiness(t *testing.T) {
	for _, agent := range []string{"codex", "Codex", ""} {
		if !IsStartupGate(folderAccessFixture, agent) {
			t.Fatal("current consent gate missed", agent)
		}
		if NotReadyReason(folderAccessFixture, agent) == "" {
			t.Fatal("consent called ready", agent)
		}
	}
}
func TestWTR004ReportsAndSettledComposersAreNotConsent(t *testing.T) {
	for _, raw := range []string{
		"report quotes Folder access and Trust this folder?", strings.ReplaceAll(folderAccessFixture, "Folder access", "quoted: Folder access"),
		folderAccessFixture + "\n› Ask Codex to do anything", folderAccessFixture + strings.Repeat("\nold", 30) + "\n›",
		"\x1b[2m" + folderAccessFixture + "\x1b[22m",
	} {
		if IsStartupGate(raw, "codex") {
			t.Fatalf("prose, stale or faint capture became consent: %q", raw)
		}
	}
}
