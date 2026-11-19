package dispatchbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/codexrpc"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

const overlapDeliveryID = "stable-id"

func TestMSD011MismatchedPendingClockRefusesBeforeRPC(t *testing.T) {
	request, indexPath := prepareOverlapRequest(t, time.Now())
	pendingPath := filepath.Join(agentclient.InputDir(), request.ID+".pending.json")
	pending := readOverlapFile(t, pendingPath)
	metadata := request
	metadata.Text = ""
	metadata.CreatedAt -= 21000
	index, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, index, 0600); err != nil {
		t.Fatal(err)
	}

	calls, submissions := 0, 0
	codexUserInputs = func(_ context.Context, _ codexrpc.Target, _ string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
		calls++
		if check() {
			submissions++
			return codexrpc.Outcome{Accepted: true, Attempted: true, StartedTurnID: "synthetic-turn"}
		}
		return codexrpc.Outcome{Reason: "target-changed"}
	}

	opts := dispatch.Opts{DirectUserInput: true, DeliveryID: overlapDeliveryID}
	result := codexNativeSubmit(request.Pane)(opts, request.Text, nil)
	if result.State != dispatch.StateFailed || result.Evidence != "codex-input-receipt-unconfirmed" || result.Attempts != 0 || calls != 0 || submissions != 0 {
		t.Fatalf("mismatched records did not fail before RPC: result=%+v calls=%d submissions=%d", result, calls, submissions)
	}
	assertOverlapFiles(t, indexPath, pendingPath)
	if !bytes.Equal(readOverlapFile(t, indexPath), index) || !bytes.Equal(readOverlapFile(t, pendingPath), pending) {
		t.Fatal("refused retry changed the mismatched records")
	}
}

func TestMSD011DelayedBridgeAndSameIDRetryCannotBothSubmit(t *testing.T) {
	request, indexPath := prepareOverlapRequest(t, time.Now().Add(-21*time.Second))
	prefix := filepath.Join(agentclient.InputDir(), request.ID)
	send := codexNativeSubmit(request.Pane)
	opts := dispatch.Opts{DirectUserInput: true, DeliveryID: overlapDeliveryID}
	var firstCheck func() bool
	var retry dispatch.Result
	calls, claims, submissions := 0, 0, 0

	codexUserInputs = func(_ context.Context, _ codexrpc.Target, id string, _ []codexrpc.Input, check func() bool) codexrpc.Outcome {
		calls++
		if id != request.ID {
			t.Fatalf("RPC entry %d used request %q, want %q", calls, id, request.ID)
		}
		switch calls {
		case 1:
			// Pause the first caller before it claims. The retry must reach
			// the same boundary and claim while the first caller is pending.
			firstCheck = check
			retry = send(opts, request.Text, nil)
			return codexrpc.Outcome{Reason: "target-changed"}
		case 2:
			if !check() {
				t.Fatal("retry did not claim the original pending request")
			}
			claims++
			// Check the first caller while the claim exists and before a
			// receipt could independently prevent another submission.
			assertOverlapFiles(t, indexPath, prefix+".claimed.json")
			if firstCheck() {
				t.Fatal("first caller acquired authority after the retry claimed")
			}
			submissions++
			return codexrpc.Outcome{Accepted: true, Attempted: true, StartedTurnID: "synthetic-turn"}
		default:
			t.Fatalf("unexpected RPC entry %d", calls)
			return codexrpc.Outcome{}
		}
	}

	first := send(opts, request.Text, nil)
	if calls != 2 || claims != 1 || submissions != 1 {
		t.Fatalf("want two RPC entries, one claim and one submission: calls=%d claims=%d submissions=%d", calls, claims, submissions)
	}
	if retry.State != dispatch.StateQueued || retry.Delivery != "started-turn" || retry.Evidence != "codex-turn-accepted:synthetic-turn" || retry.Attempts != 1 {
		t.Fatalf("claim winner did not record acceptance: %+v", retry)
	}
	if first.State != dispatch.StateQueued || first.Delivery != "started-turn" || first.Evidence != "codex-turn-already-accepted" || first.Attempts != 0 {
		t.Fatalf("first caller did not reuse the winner's receipt: %+v", first)
	}
	receipt, ok := agentclient.ReadReceipt(request)
	if !ok || receipt.ID != request.ID || receipt.State != "accepted" || receipt.Delivery != "started-turn" {
		t.Fatalf("missing accepted receipt for original request: %+v", receipt)
	}
	assertOverlapFiles(t, indexPath, prefix+".receipt.json")
	if again := send(opts, request.Text, nil); again != first || calls != 2 {
		t.Fatalf("settled retry entered RPC or returned a different receipt: %+v, calls=%d", again, calls)
	}
}

func prepareOverlapRequest(t *testing.T, createdAt time.Time) (agentclient.Request, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())

	oldTarget, oldUser := codexInputTarget, codexUserInputs
	oldQueue, oldImage, oldWake := codexQueueInputs, codexImageInputs, codexIdleWakeInputs
	t.Cleanup(func() {
		codexInputTarget, codexUserInputs = oldTarget, oldUser
		codexQueueInputs, codexImageInputs, codexIdleWakeInputs = oldQueue, oldImage, oldWake
	})
	unexpected := func(context.Context, codexrpc.Target, string, []codexrpc.Input, func() bool) codexrpc.Outcome {
		t.Fatal("unexpected native transport in isolated fixture")
		return codexrpc.Outcome{}
	}
	codexQueueInputs, codexImageInputs, codexIdleWakeInputs = unexpected, unexpected, unexpected

	f := resume.CodexFrontend{
		Instance: strings.Repeat("a", 32), HomeID: transcript.CodexHomeID(), SessionID: "session-A",
		StartedAt: createdAt.Add(-time.Second).UnixMilli(), BackendPID: 200, BackendStart: "backend",
	}
	f.Pane, f.PID, f.RootStart = "%9", 101, "root"
	codexInputTarget = func(pane string) (resume.CodexFrontend, bool) { return f, pane == f.Pane }
	attachment := agentclient.Attachment{
		Version: agentclient.ProtocolVersion, Pane: f.Pane, PID: f.PID, Instance: f.Instance,
		HomeID: f.HomeID, SessionID: f.SessionID, ServerEpoch: f.RootStart,
		BindingEpoch: f.StartedAt, InputReady: true,
	}
	request, err := agentclient.Prepare(attachment, "synthetic input", overlapDeliveryID, false, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(agentclient.InputDir(), "*.index.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("expected one prepared index: paths=%v err=%v", paths, err)
	}
	indexPath := paths[0]
	pendingPath := filepath.Join(agentclient.InputDir(), request.ID+".pending.json")
	assertOverlapFiles(t, indexPath, pendingPath)

	var index, pending agentclient.Request
	if err := json.Unmarshal(readOverlapFile(t, indexPath), &index); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readOverlapFile(t, pendingPath), &pending); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var persisted agentclient.Request
	if err := json.Unmarshal(encoded, &persisted); err != nil {
		t.Fatal(err)
	}
	metadata := persisted
	metadata.Text = ""
	if index != metadata || pending != persisted || request.CreatedAt != createdAt.UnixMilli() {
		t.Fatalf("Prepare did not publish the matching records at %s: index=%+v pending=%+v", createdAt, index, pending)
	}
	return request, indexPath
}

func readOverlapFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertOverlapFiles(t *testing.T, paths ...string) {
	t.Helper()
	entries, err := os.ReadDir(agentclient.InputDir())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	for i, path := range paths {
		paths[i] = filepath.Base(path)
	}
	slices.Sort(paths)
	if !slices.Equal(names, paths) {
		t.Fatalf("spool files = %v, want %v", names, paths)
	}
}
