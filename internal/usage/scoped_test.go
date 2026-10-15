package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestScopedAccountingKeepsBaseRolloutIdentityAndReadsAppends(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "15")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	meta := `{"type":"session_meta","payload":{"id":"accounting","originator":"codex-tui"}}` + "\n"
	base := filepath.Join(dir, "rollout-2026-10-15T00-00-00-accounting.jsonl")
	continuation := filepath.Join(dir, "rollout-2026-10-15T01-00-00-accounting_instance.jsonl")
	for path, out := range map[string]int64{base: 9, continuation: 99} {
		if err := os.WriteFile(path, []byte(meta+codexCount(now.Format(time.RFC3339Nano), 100, out)+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scope := transcript.NewCodexPathScope()
	first, ok := ForSessionScoped("codex", "accounting", now, scope)
	if !ok || first.OutTok != 9 {
		t.Fatalf("accounting switched to the continuation: %+v %v", first, ok)
	}
	file, err := os.OpenFile(base, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(codexCount(now.Add(time.Second).Format(time.RFC3339Nano), 200, 40) + "\n")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	next, ok := ForSessionScoped("codex", "accounting", now.Add(time.Second), scope)
	if !ok || next.OutTok != 40 {
		t.Fatalf("scoped accounting cached facts: %+v %v", next, ok)
	}
	if scope.DiscoveryCount() != 1 {
		t.Fatalf("accounting rediscovered: %d", scope.DiscoveryCount())
	}
}
