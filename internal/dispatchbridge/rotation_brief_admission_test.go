package dispatchbridge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestBoundWakeFinalAdmissionPreservesTypedProofAndDraftGuard(t *testing.T) {
	for _, mode := range []string{"proof-gap", "changed-target", "new-draft"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			oldTarget, oldWake := codexInputTargetReadiness, codexIdleWakeInputs
			t.Cleanup(func() { codexInputTargetReadiness, codexIdleWakeInputs = oldTarget, oldWake })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "original",
				StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "root"
			lookups, guardCalls := 0, 0
			codexInputTargetReadiness = func(string) (resume.CodexFrontend, error) {
				lookups++
				if lookups == 2 && mode != "new-draft" {
					checked := f
					if mode == "changed-target" {
						checked.SessionID = "different"
					}
					return checked, &resume.FrontendBindingError{Code: "proof-expired", Retryable: true}
				}
				return f, nil
			}
			codexIdleWakeInputs = func(_ context.Context, _ codexrpc.Target, _ string, _ []codexrpc.Input, admit func() bool) codexrpc.Outcome {
				if admit() {
					t.Fatal("unsafe briefing admitted to native input")
				}
				return codexrpc.Outcome{Reason: "target-changed"}
			}
			opts := dispatch.Opts{DeliveryID: "bound-briefing", IdleWakeInput: true, ExpectedSessionID: f.SessionID,
				ExpectedInstance: f.Instance, ExpectedHomeID: f.HomeID, ExpectedClientPID: f.PID}
			result := codexNativeSubmit(f.Pane)(opts, "synthetic briefing", func() string {
				guardCalls++
				if lookups != 2 {
					t.Fatal("final guard ran before the target check")
				}
				return "rotation-brief-composer-unavailable"
			})
			want := "target-changed"
			if mode == "proof-gap" {
				want = codexFrontendProofUnavailable
			}
			if result.Submission != dispatch.SubmissionNotSubmitted || result.Evidence != want || lookups != 2 ||
				(mode == "new-draft") != (guardCalls == 1) {
				t.Fatalf("wrong admission result: %+v, lookups=%d guards=%d", result, lookups, guardCalls)
			}
		})
	}
}
