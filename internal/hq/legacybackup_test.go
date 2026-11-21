package hq

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestRepairMissingEntryKeepsLocalAndEarlierLegacyBackup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	backup := filepath.Join(state.HQHome(), "CLAUDE.md.bak-legacy-20200101")
	for path, body := range map[string]string{
		backup: "Original policies", hqClaudePointerPath(): hqClaudePointer, hqLocalPath(): "My long-standing customization",
	} {
		if err := state.WritePrivateFile(path, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := seedHQHome("en"); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		backup: "Original policies", hqInstructionsPath(): playbookIn("en"),
		hqClaudePointerPath(): hqClaudePointer, hqLocalPath(): "My long-standing customization",
	} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("%s: repaired the wrong content or changed an original: %v", path, err)
		}
	}
}
