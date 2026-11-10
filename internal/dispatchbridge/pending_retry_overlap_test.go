package dispatchbridge

import (
	"context"
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Advance only the private index's creation clock, representing the first
// handler paused after Prepare and before starting its native RPC deadline.
func TestMSD011DelayedBridgeAndSameIDRetryCannotBothSubmit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	oldTarget, oldUser := codexInputTarget, codexUserInputs
	defer func() { codexInputTarget = oldTarget; codexUserInputs = oldUser }()
	f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "session-A", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
	f.Pane = "%9"
	f.PID = 101
	f.RootStart = "root"
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	submit := codexNativeSubmit("%9")
	opts := dispatch.Opts{DirectUserInput: true, DeliveryID: "stable-id"}
	calls, mutations := 0, 0
	var ids []string
	codexUserInputs = func(_ context.Context, target codexrpc.Target, id string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
		calls++
		ids = append(ids, id)
		if calls == 1 {
			paths, e := filepath.Glob(filepath.Join(agentclient.InputDir(), "*.index.json"))
			if e != nil || len(paths) != 1 {
				t.Fatal(paths, e)
			}
			b, e := os.ReadFile(paths[0])
			if e != nil {
				t.Fatal(e)
			}
			var req agentclient.Request
			if e = json.Unmarshal(b, &req); e != nil {
				t.Fatal(e)
			}
			req.CreatedAt -= 21000
			b, e = json.Marshal(req)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(paths[0], b, 0600); e != nil {
				t.Fatal(e)
			}
			retry := submit(opts, "synthetic", nil)
			t.Logf("retry=%+v", retry)
		}
		if !check() {
			return codexrpc.Outcome{Reason: "target-changed"}
		}
		mutations++
		return codexrpc.Outcome{Accepted: true, Attempted: true, StartedTurnID: "synthetic-turn"}
	}
	first := submit(opts, "synthetic", nil)
	t.Logf("first=%+v ids=%v mutations=%d", first, ids, mutations)
	if mutations != 1 || len(ids) != 2 || ids[0] != ids[1] {
		t.Fatal("delayed handler and same sendID retry both passed real claim and submitted")
	}
}
