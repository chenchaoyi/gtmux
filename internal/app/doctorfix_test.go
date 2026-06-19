package app

import (
	"strings"
	"testing"
)

func TestManagedKey(t *testing.T) {
	cases := map[string]string{
		"set -g set-titles on":                         "set-titles",
		"set -g @continuum-restore 'on'":               "@continuum-restore",
		"set -g set-titles-string '#S — #W'":           "set-titles-string",
		"run '~/.tmux/plugins/tpm/tpm'":                "run",
		"set -g @plugin 'tmux-plugins/tmux-resurrect'": "@plugin",
	}
	for line, want := range cases {
		if got := managedKey(line); got != want {
			t.Errorf("managedKey(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestUpsertManagedBlock(t *testing.T) {
	// Append into a non-empty config preserves the user's lines.
	out := upsertManagedBlock("set -g mouse on\n", []string{"set -g set-titles on"})
	if !strings.Contains(out, "set -g mouse on") {
		t.Error("user line was dropped")
	}
	if strings.Count(out, fixBlockBegin) != 1 || strings.Count(out, fixBlockEnd) != 1 {
		t.Errorf("want exactly one managed block, got:\n%s", out)
	}

	// Re-running replaces the block in place (idempotent — still ONE block).
	out2 := upsertManagedBlock(out, []string{"set -g set-titles on", "set -g history-limit 50000"})
	if strings.Count(out2, fixBlockBegin) != 1 {
		t.Errorf("upsert must not duplicate the block:\n%s", out2)
	}
	if !strings.Contains(out2, "history-limit 50000") || !strings.Contains(out2, "set -g mouse on") {
		t.Errorf("upsert lost content:\n%s", out2)
	}

	// Empty config → just the block.
	if e := upsertManagedBlock("", []string{"set -g set-titles on"}); !strings.HasPrefix(e, fixBlockBegin) {
		t.Errorf("empty conf should start with the block, got:\n%s", e)
	}
}

func TestMergeManagedLines(t *testing.T) {
	// Existing block has set-titles; a later run adds history-limit. Union, no dup.
	conf := upsertManagedBlock("", []string{"set -g set-titles on"})
	merged := mergeManagedLines(conf, []string{"set -g set-titles on", "set -g history-limit 50000"})
	if len(merged) != 2 {
		t.Fatalf("want 2 merged lines (deduped), got %d: %v", len(merged), merged)
	}

	// The TPM run line is always floated last.
	conf2 := upsertManagedBlock("", []string{"run '~/.tmux/plugins/tpm/tpm'", "set -g @plugin 'x'"})
	merged2 := mergeManagedLines(conf2, []string{"set -g history-limit 50000"})
	if managedKey(merged2[len(merged2)-1]) != "run" {
		t.Errorf("run line must be last, got: %v", merged2)
	}
}

// TestTpmWiringLines guards the TPM wiring: three @plugin declarations followed
// by the run line LAST (TPM must initialize after the plugins are declared).
func TestTpmWiringLines(t *testing.T) {
	lines := tpmWiringLines()
	if managedKey(lines[len(lines)-1]) != "run" {
		t.Errorf("run line must be last, got: %v", lines)
	}
	if c := strings.Count(strings.Join(lines, "\n"), "@plugin"); c != 3 {
		t.Errorf("want 3 @plugin declarations, got %d", c)
	}
}
