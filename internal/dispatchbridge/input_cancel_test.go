package dispatchbridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL022NativeCancellationSeparatesPublicationAndMutation(t *testing.T) {
	for _, phase := range []string{"before-publication", "cancel-before-wire", "deadline-before-wire", "attempted"} {
		t.Run(phase, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			oldTarget, oldQueue := codexInputTarget, codexQueueInputs
			t.Cleanup(func() { codexInputTarget, codexQueueInputs = oldTarget, oldQueue })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: "fixture-home", SessionID: "fixture-thread",
				StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "fixture"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "fixture-root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if phase == "deadline-before-wire" {
				deadline, stop := context.WithTimeout(ctx, time.Second)
				defer stop()
				ctx = deadline
			}
			var requestID string
			codexQueueInputs = func(rpcCtx context.Context, _ codexrpc.Target, id string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
				requestID = id
				if phase == "before-publication" {
					cancel()
					if check() {
						t.Fatal("cancelled before publication obtained authority")
					}
					return codexrpc.Outcome{Reason: "input-deadline"}
				}
				if !check() {
					t.Fatal("fixture did not cross the durable claim boundary")
				}
				switch phase {
				case "attempted":
					return codexrpc.Outcome{Attempted: true, Reason: "acceptance-unconfirmed"}
				case "deadline-before-wire":
					<-rpcCtx.Done()
					if !errors.Is(rpcCtx.Err(), context.DeadlineExceeded) {
						t.Fatal(rpcCtx.Err())
					}
				default:
					cancel()
				}
				return codexrpc.Outcome{Reason: "input-deadline"}
			}
			got := codexNativeSubmit(f.Pane)(dispatch.Opts{DeliveryID: "stable", Context: ctx}, "fixture", nil)
			if phase == "attempted" {
				if got.Submission != dispatch.SubmissionUnknown || got.Attempts != 1 {
					t.Fatalf("attempted mutation was not kept unknown: %+v", got)
				}
			} else if got.Submission != dispatch.SubmissionNotSubmitted || got.Attempts != 0 || got.Submitted() {
				t.Fatalf("zero mutation was not kept distinct: %+v", got)
			}
			if phase == "before-publication" {
				if _, err := os.Stat(state.Dir()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("pre-publication refusal created state: %v", err)
				}
				return
			}
			name := filepath.Join(agentclient.InputDir(), requestID+".receipt.json")
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			var receipt agentclient.Receipt
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			want := "refused"
			if phase == "attempted" {
				want = "uncertain"
			}
			if receipt.State != want {
				t.Fatalf("receipt %+v, want %q", receipt, want)
			}
			if want == "refused" {
				a := agentclient.Attachment{Version: agentclient.ProtocolVersion, Agent: "codex", Client: "codex", Pane: f.Pane,
					PID: f.PID, Instance: f.Instance, HomeID: f.HomeID, SessionID: f.SessionID, ServerEpoch: f.RootStart,
					BindingEpoch: f.StartedAt, InputReady: true}
				if _, err := agentclient.Prepare(a, "next", "new-id", false, time.Now().Add(96*time.Hour)); err != nil {
					t.Fatal("could not admit after cancelled request aged", err)
				}
				if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("known refusal still occupies a slot: %v", err)
				}
			}
		})
	}
}
