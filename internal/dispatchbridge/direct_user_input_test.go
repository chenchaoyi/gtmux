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

func TestMSD002DirectUserIngressPreservesReceiptAndNeverReplays(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, kind := range []string{"active-turn", "started-turn", "queued", "uncertain", "hold", "changed"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			oldTarget, oldUser, oldQueue := codexInputTarget, codexUserInputs, codexQueueInputs
			t.Cleanup(func() { codexInputTarget, codexUserInputs, codexQueueInputs = oldTarget, oldUser, oldQueue })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "root-a", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			calls, claims := 0, 0
			codexQueueInputs = func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
				t.Fatal("explicit phone input used background queue adapter")
				return codexrpc.Outcome{}
			}
			codexUserInputs = func(_ context.Context, target codexrpc.Target, id string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
				calls++
				if target.ThreadID != "root-a" || id == "" || len(input) != 1 || input[0].Text != "synthetic" {
					t.Fatal(target, id, input)
				}
				if kind == "changed" {
					f.SessionID = "other"
				}
				if !check() {
					return codexrpc.Outcome{Reason: "target-changed"}
				}
				claims++
				switch kind {
				case "uncertain":
					return codexrpc.Outcome{Attempted: true}
				case "active-turn":
					return codexrpc.Outcome{Accepted: true, Attempted: true, ActiveTurnID: "turn-a"}
				case "started-turn":
					return codexrpc.Outcome{Accepted: true, Attempted: true, StartedTurnID: "turn-b"}
				default:
					return codexrpc.Outcome{Accepted: true, Attempted: true, QueueID: "q-a"}
				}
			}
			hold := func() string {
				if kind == "hold" {
					return "approval pending"
				}
				return ""
			}
			send := codexNativeSubmit("%9")
			opts := dispatch.Opts{DeliveryID: "same-message", DirectUserInput: true}
			first := send(opts, "synthetic", hold)
			if kind == "hold" || kind == "changed" {
				if claims != 0 || first.State == dispatch.StateQueued {
					t.Fatal(first, claims)
				}
				return
			}
			second := send(opts, "synthetic", hold)
			if calls != 1 || claims != 1 {
				t.Fatal("replayed", calls, claims)
			}
			if kind == "uncertain" {
				if first.State != dispatch.StateFailed || second.State != dispatch.StateFailed {
					t.Fatal(first, second)
				}
				return
			}
			want := kind
			if want == "queued" {
				want = ""
			}
			if first.State != dispatch.StateQueued || second.State != dispatch.StateQueued || first.Delivery != want || second.Delivery != want {
				t.Fatal(first, second)
			}
		})
	}
	if DeliverOpts("%9", "codex", false, dispatch.Tuning{}).DirectUserInput {
		t.Fatal("background defaults changed")
	}
}
