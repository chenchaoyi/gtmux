package dispatchbridge

import (
	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"testing"
)

func TestTerminalReadinessWaitsForItsNativeReceiverAndGrant(t *testing.T) {
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
}
