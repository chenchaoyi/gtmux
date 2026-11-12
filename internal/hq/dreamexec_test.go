package hq

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/knowledge"
)

func TestDreamCLIModelSelectionAndIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX", "real-socket-must-not-pass")
	t.Setenv("CODEX_THREAD_ID", "parent-thread")
	for _, agent := range []string{"codex", "claude"} {
		c := knowledge.DreamChoice{Agent: agent, Model: "hq-chosen-model", Effort: "high", Reason: "Conflict needs careful review", TimeoutSeconds: 30}
		args, err := dreamCLIArgs(c, "/temporary/instructions.txt")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, c.Model) || strings.Contains(joined, "bypass") || strings.Contains(joined, "resume") {
			t.Fatal(args)
		}
		if agent == "codex" && (!strings.Contains(joined, "--ephemeral") || !strings.Contains(joined, "--sandbox read-only") || !strings.Contains(joined, "--disable multi_agent")) {
			t.Fatal(args)
		}
		if agent == "claude" && (!strings.Contains(joined, "--safe-mode") || !strings.Contains(joined, "--tools ") || !strings.Contains(joined, "--no-session-persistence")) {
			t.Fatal(args)
		}
	}
	for _, kv := range dreamCLIEnv() {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "CODEX_THREAD_ID=") {
			t.Fatal("inherited parent identity", kv)
		}
	}
}

func TestDreamCLIParsesTerminalUsageAndRejectsTools(t *testing.T) {
	var codex dreamStream
	if err := codex.line("codex", []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"{}"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := codex.line("codex", []byte(`{"type":"turn.completed","usage":{"input_tokens":120,"cached_input_tokens":50,"output_tokens":30}}`)); err != nil {
		t.Fatal(err)
	}
	if !codex.done || !codex.reply.UsageKnown || codex.reply.InputTokens != 120 || codex.reply.GenerationTokens != 30 {
		t.Fatal(codex)
	}
	var claude dreamStream
	_ = claude.line("claude", []byte(`{"type":"result","subtype":"success","result":"{}","usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":30,"cache_creation_input_tokens":40}}`))
	if !claude.done || claude.failed || claude.reply.InputTokens != 80 {
		t.Fatal(claude)
	}
	var unknown dreamStream
	_ = unknown.line("claude", []byte(`{"type":"result","subtype":"success","result":"{}"}`))
	if unknown.reply.UsageKnown {
		t.Fatal("missing usage became zero")
	}
	if err := codex.line("codex", []byte(`{"type":"item.started","item":{"type":"command_execution"}}`)); err == nil {
		t.Fatal("tool event accepted")
	}
	if err := claude.line("claude", []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use"}]}}`)); err == nil {
		t.Fatal("tool event accepted")
	}
}

func TestDreamCLIProcessCompletionCancellationAndBoundedOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.sh")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	message, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]string{"type": "agent_message", "text": "{}"}})
	write("cat >/dev/null\nprintf '%s\\n' '" + string(message) + "' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":3}}'\n")
	r, err := runDreamProcess(context.Background(), []string{path}, dir, []byte("authorized synthetic item"), "codex")
	if err != nil || r.Text != "{}" || !r.UsageKnown {
		t.Fatal(r, err)
	}
	write("cat >/dev/null\nsleep 20\n")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := runDreamProcess(ctx, []string{path}, dir, nil, "codex"); err == nil || time.Since(start) > 3*time.Second {
		t.Fatal("cancel did not bound the child group", err)
	}
	write("cat >/dev/null\nprintf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"{}\"}}'\n")
	if _, err := runDreamProcess(context.Background(), []string{path}, dir, nil, "codex"); err == nil {
		t.Fatal("exit zero without terminal receipt became success")
	}
	write("cat >/dev/null\nyes a | head -c 300000\n")
	if _, err := runDreamProcess(context.Background(), []string{path}, dir, nil, "codex"); err == nil {
		t.Fatal("oversized line accepted")
	}
}
