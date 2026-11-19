package dispatchbridge

import (
	"context"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSPL003CapacityRefusesBeforeRPC(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := codexInputTarget
	defer func() { codexInputTarget = old }()
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return resume.CodexFrontend{}, true }
	dir := agentclient.InputDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4096; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprint(i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	result := codexNativeSubmit("%9")(dispatch.Opts{}, "fixture", nil)
	if result.Evidence != "input-spool-full" || result.Attempts != 0 || result.FailureCode != dispatch.InputSpoolFull || result.Submission != dispatch.SubmissionNotSubmitted {
		t.Fatalf("unexpected admission result: %+v", result)
	}
}

func TestSPL008CodexSeparatesRefusalFromUnresolvedSubmission(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		attempted    bool
		code         dispatch.InputFailureCode
		boundary     dispatch.Submission
	}{
		{"question before mutation", "input-awaiting-confirmation", false, dispatch.InputQuestion, dispatch.SubmissionNotSubmitted},
		{"target changed", "target-changed", false, dispatch.InputTargetChanged, dispatch.SubmissionNotSubmitted},
		{"unconfirmed is still unknown with a refusal-like reason", "input-thread-not-loaded", true, dispatch.InputOutcomeUnconfirmed, dispatch.SubmissionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			target, queue := codexInputTarget, codexQueueInputs
			t.Cleanup(func() { codexInputTarget, codexQueueInputs = target, queue })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: "fixture-home",
				SessionID: "fixture-thread", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			calls := 0
			codexQueueInputs = func(_ context.Context, _ codexrpc.Target, _ string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
				calls++
				if tc.attempted && !check() {
					t.Fatal("fixture did not reserve before simulating delivery")
				}
				return codexrpc.Outcome{Attempted: tc.attempted, Reason: tc.reason}
			}
			submit := codexNativeSubmit("%9")
			opts := dispatch.Opts{DeliveryID: "same-fixture"}
			first := submit(opts, "fixture", nil)
			wantAttempts := 0
			if tc.attempted {
				wantAttempts = 1
			}
			if first.FailureCode != tc.code || first.Submission != tc.boundary || first.Submitted() ||
				first.Attempts != wantAttempts || calls != 1 {
				t.Fatalf("result=%+v calls=%d", first, calls)
			}
			if tc.attempted {
				second := submit(opts, "fixture", nil)
				if second.Submission != dispatch.SubmissionUnknown || second.Attempts != 0 || calls != 1 {
					t.Fatalf("unresolved request replayed: result=%+v calls=%d", second, calls)
				}
			}
		})
	}
}

func TestSPL014MissingCodexFrontendKeepsPriorOutcomeUnknown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	restore := codexInputTarget
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return resume.CodexFrontend{}, false }
	t.Cleanup(func() { codexInputTarget = restore })
	target := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "codex", Pane: "%9",
		PID: 123, Instance: strings.Repeat("a", 32), HomeID: "root", SessionID: "thread",
		ServerEpoch: "17", BindingEpoch: 9, InputReady: true}
	const id = "codex-previous"
	r, err := agentclient.Prepare(target, "fixture", id, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := agentclient.ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	submit := codexNativeSubmit("%9")
	got := submit(dispatch.Opts{DeliveryID: id}, "fixture", nil)
	if got.State != dispatch.StateFailed || got.Submission != dispatch.SubmissionUnknown ||
		got.FailureCode != dispatch.InputAssociationMissing || got.Attempts != 0 {
		t.Fatalf("prior request was treated as freshly refused: %+v", got)
	}
	fresh := submit(dispatch.Opts{DeliveryID: "genuinely-new"}, "fixture", nil)
	if fresh.State != dispatch.StateRefusedWaiting || fresh.Submission != dispatch.SubmissionNotSubmitted {
		t.Fatalf("fresh refusal lost its proof: %+v", fresh)
	}
}
