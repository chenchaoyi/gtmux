package radar

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestNativeCodexReconciliationUsesSubmittedWitness(t *testing.T) {
	for _, tamper := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-ID hook recovers from rollout", true: "similar prompt does not bind"}[tamper], func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Setenv("TMUX", "")
			t.Setenv("TMUX_TMPDIR", t.TempDir())
			cwd, now := t.TempDir(), time.Now().Unix()
			target := resume.CodexBindingTarget{Pane: "%27", Loc: "worker:0.0", Cwd: cwd, PID: 58990, PanePID: 58325}
			wire, err := resume.PrepareCodexBinding(target, "Repair test issue", now)
			if err != nil {
				t.Fatal(err)
			}
			if tamper {
				wire = "Another " + wire
			}
			dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "03")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, sid := range []string{"worker-session", "desktop-session"} {
				origin := "codex-tui"
				if sid == "desktop-session" {
					origin = "Codex Desktop"
				}
				meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": sid, "originator": origin, "cwd": cwd}})
				msg, _ := json.Marshal(map[string]any{"timestamp": time.Now().UTC().Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]string{"type": "user_message", "message": wire}})
				if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-03T00-00-00-"+sid+".jsonl"), append(append(meta, '\n'), append(msg, '\n')...), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := native.Save(native.Record{Agent: "codex", SessionID: sid, Cwd: cwd, State: "working", UpdatedAt: now, PID: os.Getpid()}); err != nil {
					t.Fatal(err)
				}
			}
			stub := filepath.Join(t.TempDir(), "tmux")
			t.Setenv("FAKE_PANE_CWD", cwd)
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s\\t%s\\t%s\\t%s\\t%s\\n' '%27' 'worker:0.0' \"$FAKE_PANE_CWD\" 58325 codex\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FAKE_CLIENT_PID", "58990")
			ps := filepath.Join(filepath.Dir(stub), "ps")
			if err := os.WriteFile(ps, []byte("#!/bin/sh\nprintf '%s\\n' '58325 1 bash' \"$FAKE_CLIENT_PID 58325 codex\"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", filepath.Dir(stub)+string(os.PathListSeparator)+os.Getenv("PATH"))
			// Run each stub once first: macOS assesses a newly written executable on its
			// first run, which under a loaded `go test ./...` can take seconds, and
			// LiveCodexBindingTarget gives ps two. Unwarmed, the Codex-binding tests
			// timed out under a full `make check` on an Intel Mac.
			for _, bin := range []string{stub, ps} {
				_ = exec.Command(bin).Run()
			}
			old := tmux.Bin
			tmux.Bin = stub
			t.Cleanup(func() { tmux.Bin = old })
			if err := resume.Save("dev:0.0", resume.Record{Agent: "codex", SessionID: "peer-session", Cwd: cwd}); err != nil {
				t.Fatal(err)
			}
			panes := []Pane{{PaneID: "%18", Loc: "dev:0.0", Agent: "Codex", cwd: cwd}, {PaneID: "%27", Loc: target.Loc, Agent: "Codex", cwd: cwd}}
			rows := nativePanes(panes, nil, now)
			wantRows := 1
			if tamper {
				wantRows = 2
			}
			if len(rows) != wantRows {
				t.Fatalf("native rows = %d, want %d", len(rows), wantRows)
			}
			rec, ok := resume.Load(target.Loc)
			if tamper && ok || !tamper && (!ok || rec.SessionID != "worker-session") {
				t.Fatal("wrong worker binding", rec, ok)
			}
			if !tamper {
				if rows[0].sessionID != "desktop-session" {
					t.Fatal("desktop session was absorbed into tmux")
				}
				turns, err := transcript.Load(rec.Agent, rec.SessionID, 10)
				if err != nil || len(turns) != 1 || turns[0].Prompt != "Repair test issue" {
					t.Fatal("binding did not restore clean worker Chat", turns, err)
				}
			}
			if peer, _ := resume.Load("dev:0.0"); peer.SessionID != "peer-session" {
				t.Fatal("same-directory peer's owner changed")
			}
		})
	}
}
