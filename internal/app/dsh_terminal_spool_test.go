package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
)

func TestDSHTerminalReceiverSharesSpoolWithGo(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DSH_HOME", filepath.Join(home, ".dsh"))
	t.Setenv("GTMUX_LANG", "en")
	inst := agentInstallers["dsh"]
	if err := updateDSHPlugin(inst, "/opt/gtmux", true); err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst", "bridge.mjs")
	now := time.Now()
	target := agentclient.Attachment{
		Version: agentclient.ProtocolVersion, Agent: "dsh", Client: "dst", Pane: "%55",
		Instance: strings.Repeat("a", 32), HomeID: "dsh-home", SessionID: "dsh-thread",
		PID: 101, ServerEpoch: "88", BindingEpoch: now.Add(-time.Second).UnixMilli(),
		InputReady: true, NativeSession: true,
	}
	dsh, err := agentclient.Prepare(target, "synthetic DSH", "dsh-action", false, now)
	if err != nil {
		t.Fatal(err)
	}
	dir := agentclient.InputDir()
	// Two in-flight records each reserve four names during receipt publication.
	for n := 0; n < 4088; n++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("foreign-%d", n)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(dsh)
	if err != nil {
		t.Fatal(err)
	}
	const script = `
import assert from 'node:assert/strict';
import {readSync, writeSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
const [module, dir, data] = process.argv.slice(1);
const {receive} = await import(pathToFileURL(module));
const request = JSON.parse(data);
let submissions = 0;
const agent = {
  session: {id: request.session_id, header: {version: 4}},
  followup(message) {
    submissions++;
    assert.equal(message.id, request.request_id);
    assert.equal(message.content[0].text, request.text);
    writeSync(1, 'claimed\n');
    const signal = Buffer.alloc(1);
    assert.equal(readSync(0, signal, 0, 1, null), 1);
    assert.equal(signal.toString(), 'x');
  }
};
const invocation = {rawInput: request.request_id, agent, signal: new AbortController().signal};
assert.equal(receive(dir, request, invocation, request.created_at + 1000).kind, 'success');
assert.equal(receive(dir, request, invocation, request.created_at + 1000).kind, 'success');
assert.equal(submissions, 1);
`
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module", "-e", script, module, dir, string(data))
	resume, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			cancel()
			_ = cmd.Wait()
		}
	}()
	ready := bufio.NewScanner(stdout)
	if !ready.Scan() || ready.Text() != "claimed" {
		t.Fatalf("legacy receiver did not enter the claim window: marker=%q err=%v", ready.Text(), ready.Err())
	}
	if _, ok := agentclient.ReadReceipt(dsh); ok {
		t.Fatal("legacy receipt appeared before followup returned")
	}
	if _, err := agentclient.Prepare(target, dsh.Text, "dsh-action", false, now); !errors.Is(err, agentclient.ErrInputUncertain) {
		t.Fatalf("legacy claim did not block replay while the receipt was pending: %v", err)
	}

	codex := target
	codex.Agent, codex.Client, codex.Pane = "codex", "codex", "%56"
	codex.Instance, codex.HomeID, codex.SessionID, codex.PID = strings.Repeat("b", 32), "codex-home", "codex-thread", 102
	direct, err := agentclient.Prepare(codex, "synthetic Go", "go-action", false, now)
	if err != nil {
		t.Fatalf("reserve final operation while legacy receiver completes: %v", err)
	}
	if _, err := agentclient.Prepare(codex, "capacity fixture", "over-limit", false, now); !errors.Is(err, agentclient.ErrInputFull) {
		t.Fatalf("over-capacity operation was not refused: %v", err)
	}
	if err := agentclient.ClaimNative(direct); err != nil {
		t.Fatal(err)
	}
	if err := agentclient.FinishNativeDelivery(direct, "accepted", "", "started-turn"); err != nil {
		t.Fatalf("Go receipt could not settle at capacity: %v", err)
	}
	if receipt, ok := agentclient.ReadReceipt(direct); !ok || receipt.State != "accepted" || receipt.Delivery != "started-turn" {
		t.Fatalf("Go receipt missing or corrupt: %+v", receipt)
	}

	if _, err := io.WriteString(resume, "x"); err != nil {
		t.Fatal(err)
	}
	if err := resume.Close(); err != nil {
		t.Fatal(err)
	}
	if ready.Scan() || ready.Err() != nil {
		t.Fatalf("unexpected legacy receiver output after settlement: %q, %v", ready.Text(), ready.Err())
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("legacy receiver did not settle and replay its receipt: %v\n%s", err, stderr.String())
	}
	if receipt, ok := agentclient.ReadReceipt(dsh); !ok || receipt.State != "accepted" {
		t.Fatalf("Go could not read legacy acceptance: %+v", receipt)
	}
	if again, err := agentclient.Prepare(target, dsh.Text, "dsh-action", false, now.Add(2*time.Second)); err != nil || again.ID != dsh.ID {
		t.Fatalf("legacy receipt did not preserve request identity: %+v, %v", again, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 4093 {
		t.Fatalf("unexpected completed spool occupancy: %d entries, %v", len(entries), err)
	}
}
