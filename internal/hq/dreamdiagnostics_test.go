package hq

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDreamDiagnosticsClassifiedBoundedAndPrivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const secret = "private-credential-and-session-url-marker"
	var warning dreamStream
	for _, event := range []string{
		`{"type":"item.completed","item":{"type":"error","message":"Code Mode is unavailable because code-mode host is disabled."}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"{}"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`,
	} {
		if err := warning.line("codex", []byte(event)); err != nil {
			t.Fatal("nonterminal diagnostic misclassified as tool", err)
		}
	}
	if warning.failed || !warning.done || !warning.reply.UsageKnown || warning.reply.FailureCode != "" {
		t.Fatal(warning)
	}
	if err := warning.line("codex", []byte(`{"type":"item.started","item":{"type":"command_execution"}}`)); err == nil {
		t.Fatal("actual tool accepted")
	}
	for _, tc := range []struct{ agent, event string }{
		{"codex", `{"type":"error","message":"401 unauthorized ` + secret + `"}`},
		{"codex", `{"type":"turn.failed","error":{"message":"authentication failed ` + secret + `"}}`},
		{"claude", `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"authentication failed ` + secret + `"}`},
	} {
		var s dreamStream
		if err := s.line(tc.agent, []byte(tc.event)); err != nil || !s.failed || s.reply.FailureCode != "authentication" {
			t.Fatalf("%s failure lost: %+v %v", tc.agent, s, err)
		}
		b, _ := json.Marshal(s.reply)
		if strings.Contains(string(b), secret) {
			t.Fatal("provider diagnostics leaked")
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.sh")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("#!/bin/sh\ncat >/dev/null\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("printf '%s' 'unknown feature " + secret + "' >&2\nexit 2\n")
	r, err := runDreamProcess(context.Background(), []string{path}, dir, nil, "codex")
	if err == nil || r.FailureCode != "invalid_config" || r.ExitCode == nil || *r.ExitCode != 2 || strings.Contains(err.Error(), secret) {
		t.Fatal(r, err)
	}
	write("yes a | head -c 200000 >&2\nprintf '%s' 'connection refused " + secret + "' >&2\nexit 1\n")
	r, err = runDreamProcess(context.Background(), []string{path}, dir, nil, "codex")
	if err == nil || r.FailureCode != "network" {
		t.Fatal(r, err)
	}
	var d dreamDiagnostics
	if n, err := d.Write([]byte(strings.Repeat("a", 200000))); err != nil || n != 200000 || len(d.tail) != 8192 {
		t.Fatal(n, err, len(d.tail))
	}
	_, _ = d.Write([]byte("end"))
	if len(d.tail) != 8192 || !strings.HasSuffix(string(d.tail), "end") {
		t.Fatal("capture not bounded")
	}
	write("sleep 20\n")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r, err = runDreamProcess(ctx, []string{path}, dir, nil, "codex")
	if err == nil || r.FailureCode != "cancelled" {
		t.Fatal(r, err)
	}
	write("exit 0\n")
	r, err = runDreamProcess(context.Background(), []string{path}, dir, nil, "codex")
	if err == nil || r.FailureCode != "incomplete" || *r.ExitCode != 0 {
		t.Fatal(r, err)
	}
}
