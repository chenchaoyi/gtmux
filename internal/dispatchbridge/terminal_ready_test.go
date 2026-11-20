package dispatchbridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestTerminalReadinessWaitsForItsNativeReceiverAndGrant(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Join(state.Dir(), "terminal-input"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		agent          string
		present, input bool
		want           bool
	}{{"dsh", false, false, false}, {"dsh", true, false, false}, {"dsh", true, true, true}, {"codex", false, false, true}} {
		lookup := func(string) (agentclient.Attachment, bool) {
			return agentclient.Attachment{Agent: "dsh", InputReady: tt.input}, tt.present
		}
		if got := terminalTransportReady("%1", tt.agent, lookup); got != tt.want {
			t.Fatalf("%+v ready=%v", tt, got)
		}
	}
	if err := os.WriteFile(filepath.Join(state.Dir(), "terminal-input-cutover.json"), []byte(`{"version":1,"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	lookup := func(string) (agentclient.Attachment, bool) {
		return agentclient.Attachment{Agent: "dsh", InputReady: true}, true
	}
	if terminalTransportReady("%1", "dsh", lookup) {
		t.Fatal("startup admitted a fresh old heartbeat against a corrupt store")
	}
}
