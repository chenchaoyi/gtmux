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

func TestHRW002ActualNativeWakePolicyKeepsTargetAndAtMostOnce(t *testing.T) {
	for _, state := range []string{"accepted", "unknown", "busy", "changed"} {
		t.Run(state, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			oldTarget, oldIdle, oldQueue := codexInputTarget, codexIdleWakeInputs, codexQueueInputs
			defer func() { codexInputTarget, codexIdleWakeInputs, codexQueueInputs = oldTarget, oldIdle, oldQueue }()
			f := resume.CodexFrontend{Instance: strings.Repeat("c", 32), HomeID: transcript.CodexHomeID(), SessionID: "root-a", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			codexQueueInputs = func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
				t.Fatal("wake fell into old queue policy")
				return codexrpc.Outcome{}
			}
			calls, mutations := 0, 0
			codexIdleWakeInputs = func(_ context.Context, target codexrpc.Target, id string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
				calls++
				if target.ThreadID != "root-a" || target.HomeID != f.HomeID || target.BackendPID != 200 || id == "" || len(input) != 1 {
					t.Fatal("native target/input changed", target, input)
				}
				if state == "busy" {
					return codexrpc.Outcome{Reason: "wake-loaded-idle-required"}
				}
				if state == "changed" {
					f.SessionID = "foreign"
				}
				if !check() {
					return codexrpc.Outcome{Reason: "target-changed"}
				}
				mutations++
				if state == "unknown" {
					return codexrpc.Outcome{Attempted: true, Reason: "acceptance-unconfirmed"}
				}
				return codexrpc.Outcome{Accepted: true, Attempted: true, AcknowledgedTurnID: "rpc-returned-turn"}
			}
			send := codexNativeSubmit("%9")
			opts := dispatch.Opts{DeliveryID: "stable-wake-id", IdleWakeInput: true}
			first := send(opts, "synthetic wake", nil)
			if state == "accepted" || state == "unknown" {
				second := send(opts, "synthetic wake", nil)
				if calls != 1 || mutations != 1 {
					t.Fatal("possibly accepted wake automatically retried", calls, mutations)
				}
				if state == "accepted" && (first.Delivery != "wake-accepted" || second.Delivery != "wake-accepted" || first.State != dispatch.StateQueued) {
					t.Fatal(first, second)
				}
				if state == "unknown" && (first.State != dispatch.StateFailed || second.State != dispatch.StateFailed) {
					t.Fatal(first, second)
				}
			} else if mutations != 0 || first.State != dispatch.StateRefusedWaiting {
				t.Fatal("unverified/busy wake mutated", mutations, first)
			}
		})
	}
}
