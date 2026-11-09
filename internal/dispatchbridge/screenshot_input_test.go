package dispatchbridge

import (
	"context"
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
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestSSI002NativeImageAcceptanceAndUncertainRetryKeepIdentity(t *testing.T) {
	for _, mode := range []string{"active", "queue", "uncertain", "backend-changed", "held"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			target, queue, image := codexInputTarget, codexQueueInputs, codexImageInputs
			t.Cleanup(func() { codexInputTarget, codexQueueInputs, codexImageInputs = target, queue, image })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "synthetic-thread", StartedAt: time.Now().UnixMilli(), BackendPID: 201, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%901", 101, "root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			root := filepath.Join(state.Dir(), "uploads")
			if e := os.MkdirAll(root, 0700); e != nil {
				t.Fatal(e)
			}
			png := filepath.Join(root, "synthetic.png")
			if e := os.WriteFile(png, []byte{137, 80, 78, 71, 13, 10, 26, 10}, 0600); e != nil {
				t.Fatal(e)
			}
			codexQueueInputs = func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
				t.Fatal("image took text-only queue path")
				return codexrpc.Outcome{}
			}
			calls := 0
			requestID := ""
			codexImageInputs = func(_ context.Context, tar codexrpc.Target, id string, inputs []codexrpc.Input, check func() bool) codexrpc.Outcome {
				calls++
				requestID = id
				if tar.ThreadID != f.SessionID || tar.HomeID != f.HomeID || tar.BackendPID != 201 || len(inputs) != 2 || inputs[1].Type != "localImage" || inputs[1].Path != png {
					t.Fatal("target/image lost", tar, inputs)
				}
				if mode == "backend-changed" {
					f.BackendPID++
				}
				if !check() {
					return codexrpc.Outcome{Reason: "target-changed"}
				}
				if !check() {
					t.Fatal("same explicit unsupported-method fallback could not recheck claim")
				}
				switch mode {
				case "active":
					return codexrpc.Outcome{Accepted: true, Attempted: true, ActiveTurnID: "turn-a"}
				case "queue":
					return codexrpc.Outcome{Accepted: true, Attempted: true, QueueID: "q-a"}
				default:
					return codexrpc.Outcome{Attempted: true, Reason: "acceptance-unconfirmed"}
				}
			}
			hold := func() string {
				if mode == "held" {
					return "approval"
				}
				return ""
			}
			text := "Inspect screenshot\n" + png
			send := codexNativeSubmit("%901")
			opts := dispatch.Opts{DeliveryID: "synthetic-user-action", ActiveImageInput: true}
			first := send(opts, text, hold)
			if mode == "active" || mode == "queue" {
				second := send(opts, text, hold)
				if calls != 1 || first.State != dispatch.StateQueued || second.State != dispatch.StateQueued {
					t.Fatal(first, second, calls)
				}
				if mode == "active" {
					if !strings.HasPrefix(first.Evidence, "codex-active-accepted:") || second.Evidence != "codex-active-already-accepted" {
						t.Fatal(first, second)
					}
					r, ok := agentclient.ReadReceipt(agentclient.Request{ID: requestID, Instance: f.Instance, HomeID: f.HomeID, SessionID: f.SessionID, PID: f.PID})
					if !ok || r.Delivery != "active-turn" {
						t.Fatal(r, ok)
					}
				}
			} else if mode == "uncertain" {
				second := send(opts, text, hold)
				if calls != 1 || first.State != dispatch.StateFailed || second.State != dispatch.StateFailed {
					t.Fatal(first, second, calls)
				}
			} else if first.State != dispatch.StateRefusedWaiting {
				t.Fatal(first)
			}
		})
	}
}

func TestSSI002OrdinaryTextKeepsExistingQueueSemantics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	target, queue, image := codexInputTarget, codexQueueInputs, codexImageInputs
	t.Cleanup(func() { codexInputTarget, codexQueueInputs, codexImageInputs = target, queue, image })
	f := resume.CodexFrontend{Instance: strings.Repeat("b", 32), HomeID: transcript.CodexHomeID(), SessionID: "text-thread", StartedAt: time.Now().UnixMilli()}
	f.Pane, f.PID = "%902", 102
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	codexImageInputs = func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
		t.Fatal("ordinary text changed to steer")
		return codexrpc.Outcome{}
	}
	expectedInputs := 1
	codexQueueInputs = func(_ context.Context, _ codexrpc.Target, _ string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
		if len(input) != expectedInputs || !check() {
			t.Fatal(input)
		}
		return codexrpc.Outcome{Accepted: true, Attempted: true, QueueID: "text-q"}
	}
	got := codexNativeSubmit("%902")(dispatch.Opts{}, "plain HQ coordination", nil)
	if got.State != dispatch.StateQueued || !strings.HasPrefix(got.Evidence, "codex-queue-accepted:") {
		t.Fatal(got)
	}
	root := filepath.Join(state.Dir(), "uploads")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	png := filepath.Join(root, "quoted.png")
	if err := os.WriteFile(png, []byte{137, 80, 78, 71, 13, 10, 26, 10}, 0600); err != nil {
		t.Fatal(err)
	}
	expectedInputs = 2
	got = codexNativeSubmit("%902")(dispatch.Opts{}, "HQ references an existing image\n"+png, nil)
	if got.State != dispatch.StateQueued || !strings.HasPrefix(got.Evidence, "codex-queue-accepted:") {
		t.Fatal("quoted path opted in to active input", got)
	}
}
