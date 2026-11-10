package dispatchbridge

import (
	"context"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"strings"
	"testing"
	"time"
)

func TestMSD006PendingRetryCannotRetarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	oldTarget, oldUser := codexInputTarget, codexUserInputs
	defer func() { codexInputTarget = oldTarget; codexUserInputs = oldUser }()
	f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "session-A", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
	f.Pane = "%9"
	f.PID = 101
	f.RootStart = "root"
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	calls := 0
	mutatedTarget := ""
	requestID := ""
	codexUserInputs = func(_ context.Context, target codexrpc.Target, id string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
		calls++
		if calls == 1 {
			requestID = id
			return codexrpc.Outcome{Reason: "input-metadata-unavailable"}
		}
		if check() {
			mutatedTarget = target.ThreadID
			t.Logf("retry used requestID=%s sameID=%v target=%s", id, id == requestID, target.ThreadID)
			return codexrpc.Outcome{Accepted: true, Attempted: true, StartedTurnID: "turn-B"}
		}
		return codexrpc.Outcome{Reason: "target-changed"}
	}
	send := codexNativeSubmit("%9")
	opts := dispatch.Opts{DirectUserInput: true, DeliveryID: "same-id"}
	first := send(opts, "synthetic", nil)
	f.SessionID = "session-B"
	f.Instance = strings.Repeat("b", 32)
	f.StartedAt++
	second := send(opts, "synthetic", nil)
	t.Logf("first=%+v second=%+v mutatedTarget=%s", first, second, mutatedTarget)
	if mutatedTarget != "" {
		t.Fatalf("same phone retry crossed session-A to %s", mutatedTarget)
	}
}
