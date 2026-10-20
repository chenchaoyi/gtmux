package hook

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDSHUsageHookHasNoLifecycleOrConversationAuthority(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "%7")
	raw := fmt.Sprintf(`{"source_id":%q,"generation":1,"status":"ready","account_id":%q,"wallets":[{"currency":"CNY","amount":"12.3400","kind":"recharge"}],"session_id":"untrusted-session","prompt":"must not become a message"}`, strings.Repeat("b", 32), strings.Repeat("a", 64))
	if rc := Run(strings.NewReader(raw), []string{"--agent", "dsh", "--detached", "Usage"}); rc != 0 {
		t.Fatal(rc)
	}
	found := false
	err := filepath.WalkDir(state.Dir(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(state.Dir(), path)
		if strings.HasPrefix(rel, "account-usage/") {
			found = true
			return nil
		}
		if rel == "lease-dsh-account-usage.lock" {
			return nil
		}
		t.Errorf("metadata hook wrote unexpected lifecycle/conversation file: %s", rel)
		return nil
	})
	if err != nil || !found {
		t.Fatal(err, found)
	}
}
