package hook

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDSHWebCannotClaimInheritedPaneOrJournalUnconsentedPrompt(t *testing.T) {
	hermeticEnv(t)
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "%999")
	Run(strings.NewReader(`{"session_id":"root","cwd":"/project","prompt":"PRIVATE UNCONSENTED","turn_id":"1"}`), []string{"--agent", "dsh", "--detached", "UserPromptSubmit"})
	r, ok := native.Load("root", dsh.HomeID())
	if !ok || r.State != "working" || r.HomeID != dsh.HomeID() {
		t.Fatal("native observation missing", r)
	}
	if state.Exists(state.ActivePath("%999")) {
		t.Fatal("Web server inherited pane ownership")
	}
	rows := events.Read(0, 1<<62)
	for _, r := range rows {
		if r.AgentSession == "root" && (r.Pane != "" || r.HomeID != dsh.HomeID() || strings.Contains(r.Summary, "PRIVATE")) {
			t.Fatalf("scope/privacy lost: %+v", r)
		}
	}
	if got := classify("dsh", "PermissionRequest", "bash"); got.Actionable {
		t.Fatal("automatic approval treated as human wait")
	}
	if got := classify("dsh", "Question", ""); got.Kind != KindQuestion {
		t.Fatal(got)
	}
}
