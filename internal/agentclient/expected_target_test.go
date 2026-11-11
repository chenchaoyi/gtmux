package agentclient

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"os"
	"testing"
)

func TestASC004ExpectedHarnessIdentityRefusesBeforePreparingInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pasted := 0
	io := WrapIO("%9", dispatch.IO{Capture: func() string { return "" }, Paste: func(string) error { pasted++; return nil }}, func(string) (Attachment, bool) {
		return Attachment{InputReady: true, SessionID: "new", HomeID: "h", Instance: "i", PID: 77}, true
	})
	io.Begin(dispatch.Opts{ExpectedSessionID: "old", ExpectedHomeID: "h", ExpectedInstance: "i", ExpectedClientPID: 77}, "private input")
	if e := io.Paste("private input"); e == nil || pasted != 0 {
		t.Fatal("wrong input pasted", e, pasted)
	}
	if _, e := os.Stat(InputDir()); !os.IsNotExist(e) {
		t.Fatal("prepared wrong recipient", e)
	}
}
