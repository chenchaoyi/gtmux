package app

import (
	"io"
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

// TestAnswerYes guards the confirm() safety fix: Enter / y / yes = yes, but EOF
// with no input (redirected stdin) must NOT auto-confirm a mutating action.
func TestAnswerYes(t *testing.T) {
	cases := []struct {
		line    string
		err     error
		want    bool
		comment string
	}{
		{"\n", nil, true, "Enter = default yes"},
		{"y\n", nil, true, "y"},
		{"yes\n", nil, true, "yes"},
		{"Y\n", nil, true, "uppercase Y"},
		{"n\n", nil, false, "n"},
		{"no\n", nil, false, "no"},
		{"", io.EOF, false, "EOF, no input → NOT yes"},
		{"y", io.EOF, true, "y then EOF (no newline) → yes"},
	}
	for _, c := range cases {
		if got := answerYes(c.line, c.err); got != c.want {
			t.Errorf("answerYes(%q, %v) = %v, want %v (%s)", c.line, c.err, got, c.want, c.comment)
		}
	}
}

func TestCodexHooksFeatureEnabled(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"model = \"x\"\n", false},
		{"features.hooks = true\n", true},
		{"features.hooks=true\n", true},
		{"features.hooks = false\n", false},
		{"[features]\nhooks = true\n", true},
		{"[features]\nother = 1\nhooks = true\n", true},
		// hooks=true under a DIFFERENT table doesn't count.
		{"[other]\nhooks = true\n", false},
		{"[features]\nhooks = false\n", false},
	}
	for _, c := range cases {
		if got := codexHooksFeatureEnabled(c.in); got != c.want {
			t.Errorf("codexHooksFeatureEnabled(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestEnableHooksUnderFeatures pins the bug the doctor hit: when a [features]
// table already exists, --fix must actually WRITE `hooks = true` under it (not
// just print guidance), so a follow-up `doctor` reports wired.
func TestEnableHooksUnderFeatures(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		// The exact shape from the field report: a [features] table with an
		// unrelated key, followed by another table that must survive intact.
		{"insert under table", "model = \"x\"\n\n[features]\njs_repl = false\n\n[mcp_servers.node_repl]\ncmd = \"node\"\n"},
		// Flip an explicit hooks = false rather than duplicating the key.
		{"flip false", "[features]\nhooks = false\nother = 1\n"},
		// Insert when the [features] table is empty.
		{"empty table", "[features]\n"},
		// No table → dotted top-level fallback.
		{"no table", "model = \"x\"\n"},
	}
	for _, c := range cases {
		out := enableHooksUnderFeatures(c.in)
		if !codexHooksFeatureEnabled(out) {
			t.Errorf("%s: features.hooks not enabled after fix:\n%s", c.name, out)
		}
		// The foreign table + key must be preserved.
		if strings.Contains(c.in, "[mcp_servers.node_repl]") && !strings.Contains(out, "[mcp_servers.node_repl]") {
			t.Errorf("%s: dropped a foreign table:\n%s", c.name, out)
		}
		// A flip must not leave a stray `hooks = false` behind.
		if c.name == "flip false" && strings.Contains(out, "hooks = false") {
			t.Errorf("%s: left a stale hooks = false:\n%s", c.name, out)
		}
	}
}

func TestTomlHasTable(t *testing.T) {
	if !tomlHasTable("[features]\nx = 1\n", "features") {
		t.Error("should find [features]")
	}
	if tomlHasTable("features.hooks = true\n", "features") {
		t.Error("dotted key is not a [features] table")
	}
	if tomlHasTable("[features.sub]\n", "features") {
		t.Error("[features.sub] is a different table")
	}
}

func TestInsertTomlTopLevel(t *testing.T) {
	line := "notify = [\"g\"]"

	// empty file → just the line
	if got := insertTomlTopLevel("", line); got != line+"\n" {
		t.Errorf("empty: got %q", got)
	}
	// no tables → appended, key stays top-level
	got := insertTomlTopLevel("model = \"x\"\n", line)
	if !strings.Contains(got, "model = \"x\"") || !strings.Contains(got, line) {
		t.Errorf("no-table: got %q", got)
	}
	// with a table → inserted BEFORE the table header (stays top-level)
	got = insertTomlTopLevel("model=\"x\"\n[mcp.y]\nfoo=1\n", line)
	ni, ti := strings.Index(got, "notify"), strings.Index(got, "[mcp.y]")
	if ni < 0 || ti < 0 || ni > ti {
		t.Errorf("notify must come before the table header:\n%s", got)
	}
}
