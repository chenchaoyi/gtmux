package dispatchbridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCodexNativeInputKeepsExactTargetAndUncertainRetry(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "accepted", true: "uncertain"}[uncertain], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			oldTarget, oldQueue := codexInputTarget, codexQueueInputs
			t.Cleanup(func() { codexInputTarget, codexQueueInputs = oldTarget, oldQueue })
			f := resume.CodexFrontend{Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "root-a", StartedAt: time.Now().UnixMilli(), BackendPID: 200, BackendStart: "backend"}
			f.Pane, f.PID, f.RootStart = "%9", 101, "root"
			codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
			calls := 0
			codexQueueInputs = func(_ context.Context, target codexrpc.Target, id string, input []codexrpc.Input, check func() bool) codexrpc.Outcome {
				if target.ThreadID != "root-a" || target.HomeID != f.HomeID || target.BackendPID != 200 || len(input) != 1 || input[0].Text != "synthetic input" {
					t.Fatal("input retargeted", target, input)
				}
				if !check() {
					t.Fatal("claim failed")
				}
				calls++
				if uncertain {
					return codexrpc.Outcome{Attempted: true, Reason: "acceptance-unconfirmed"}
				}
				return codexrpc.Outcome{Attempted: true, Accepted: true, QueueID: "q-a"}
			}
			send := codexNativeSubmit("%9")
			opts := dispatch.Opts{DeliveryID: "same-user-action"}
			first := send(opts, "synthetic input", nil)
			second := send(opts, "synthetic input", nil)
			if calls != 1 {
				t.Fatal("native request replayed", calls)
			}
			if uncertain && (first.State != dispatch.StateFailed || second.State != dispatch.StateFailed) {
				t.Fatal(first, second)
			}
			if !uncertain && (first.State != dispatch.StateQueued || second.State != dispatch.StateQueued) {
				t.Fatal(first, second)
			}
		})
	}
}

func TestCodexQueueRechecksDestinationBeforeClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	oldTarget, oldQueue := codexInputTarget, codexQueueInputs
	t.Cleanup(func() { codexInputTarget, codexQueueInputs = oldTarget, oldQueue })
	f := resume.CodexFrontend{Instance: strings.Repeat("b", 32), HomeID: transcript.CodexHomeID(), SessionID: "old", StartedAt: time.Now().UnixMilli()}
	f.Pane, f.PID, f.RootStart = "%9", 101, "root"
	codexInputTarget = func(string) (resume.CodexFrontend, bool) { return f, true }
	mutations := 0
	codexQueueInputs = func(_ context.Context, _ codexrpc.Target, _ string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
		f.SessionID = "new"
		if check() {
			mutations++
		}
		return codexrpc.Outcome{Reason: "target-changed"}
	}
	result := codexNativeSubmit("%9")(dispatch.Opts{}, "synthetic input", nil)
	if mutations != 0 || result.State != dispatch.StateRefusedWaiting {
		t.Fatal("changed conversation received input", mutations, result)
	}
}

func TestCodexAttachmentsKeepImageTypeAndRejectOutsideLinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(state.Dir(), "uploads")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	image := filepath.Join(root, "synthetic image.png")
	outside := filepath.Join(t.TempDir(), "outside.png")
	link := filepath.Join(root, "link.png")
	for _, path := range []string{image, outside} {
		if err := os.WriteFile(path, png, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	text := "inspect the attachment\n" + image + "\n" + outside + "\n" + link
	inputs := codexInputs(text)
	if len(inputs) != 2 || inputs[0].Text != text || inputs[1].Type != "localImage" || inputs[1].Path != image {
		t.Fatal("attachment semantics changed", inputs)
	}
}

func TestManagedCodexReadyGateRequiresVerifiedFrontend(t *testing.T) {
	old := codexReadyTarget
	t.Cleanup(func() { codexReadyTarget = old })
	verified := false
	codexReadyTarget = func(string) (resume.CodexFrontend, bool) { return resume.CodexFrontend{}, verified }
	for _, v := range []bool{false, true} {
		verified = v
		g := readyGate{agent: "codex"}
		capture := func() string { return "prior output\n\n› \n\n? for shortcuts" }
		g.step(readyCommand("%9", "gtmux", "codex"), capture)
		if got := g.step(readyCommand("%9", "gtmux", "codex"), capture); got != v {
			t.Fatal("gtmux foreground confused with ready Codex", v, got)
		}
	}
}
