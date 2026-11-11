package dispatchbridge

import (
	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"os"
	"testing"
)

func TestASC004ExpectedCodexIdentityRefusesBeforePreparingInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := codexInputTarget
	t.Cleanup(func() { codexInputTarget = old })
	f := resume.CodexFrontend{SessionID: "new", HomeID: "h", Instance: "i"}
	f.PID = 77
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	r := codexNativeSubmit("%9")(dispatch.Opts{ExpectedSessionID: "old", ExpectedHomeID: "h", ExpectedInstance: "i", ExpectedClientPID: 77}, "private input", nil)
	if r.Evidence != "expected-conversation-changed" {
		t.Fatal(r)
	}
	if _, e := os.Stat(agentclient.InputDir()); !os.IsNotExist(e) {
		t.Fatal("prepared wrong recipient", e)
	}
}
