package dispatchbridge

import (
	"context"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestCFL004MissingAssociationNeverPreparesClaimsOrCallsNativeInput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	target, queue, user, image, wake := codexInputTarget, codexQueueInputs, codexUserInputs, codexImageInputs, codexIdleWakeInputs
	t.Cleanup(func() {
		codexInputTarget, codexQueueInputs, codexUserInputs, codexImageInputs, codexIdleWakeInputs = target, queue, user, image, wake
	})
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return resume.CodexFrontend{}, false }
	calls := 0
	input := func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
		calls++
		return codexrpc.Outcome{Reason: "unexpected native call"}
	}
	codexQueueInputs, codexUserInputs, codexImageInputs, codexIdleWakeInputs = input, input, input, input
	for _, opts := range []dispatch.Opts{{DeliveryID: "ordinary"}, {DeliveryID: "phone", DirectUserInput: true}, {DeliveryID: "image", ActiveImageInput: true}, {DeliveryID: "wake", IdleWakeInput: true}} {
		got := codexNativeSubmit("%9")(opts, "synthetic input", nil)
		if got.Delivered || got.Evidence != "codex-frontend-unavailable" {
			t.Fatal("missing association acquired another meaning", got)
		}
	}
	if calls != 0 {
		t.Fatal("unverified association attempted native RPC", calls)
	}
	if _, err := os.Stat(agentclient.InputDir()); !os.IsNotExist(err) {
		t.Fatal("missing association reached Prepare/Claim storage", err)
	}
}
