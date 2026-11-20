package dispatchbridge

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL016CodexNativeHoldPublishesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldTarget, oldQueue := codexInputTarget, codexQueueInputs
	t.Cleanup(func() { codexInputTarget, codexQueueInputs = oldTarget, oldQueue })
	f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: "fixture-home", SessionID: "fixture-thread",
		StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "fixture"}
	f.Pane, f.PID, f.RootStart = "%9", 101, "fixture-root"
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	codexQueueInputs = func(_ context.Context, _ codexrpc.Target, _ string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
		if check() {
			t.Fatal("blocked native gate granted authority")
		}
		return codexrpc.Outcome{Reason: "target-changed"}
	}
	r := codexNativeSubmit("%9")(dispatch.Opts{DeliveryID: "fixture-held"}, "synthetic", func() string { return "fixture hold" })
	if r.Submission != dispatch.SubmissionNotSubmitted || r.Attempts != 0 {
		t.Fatalf("native refusal was not preserved: %+v", r)
	}
	if _, err := os.Lstat(state.Dir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("native gate reserved persistent space: %v", err)
	}
}
