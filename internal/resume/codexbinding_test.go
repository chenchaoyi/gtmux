package resume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func bindingFixture(t *testing.T, originator string) (CodexBindingTarget, string, CodexBindingIntent, int64) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	target := CodexBindingTarget{Pane: "%27", Loc: "worker:0.0", Cwd: t.TempDir(), PID: 58990, PanePID: 58325}
	now := time.Now().Unix()
	wire, err := PrepareCodexBinding(target, "Repair the test issue", now)
	if err != nil {
		t.Fatal(err)
	}
	intent, ok := CodexBindingForPrompt(wire, now)
	if !ok {
		t.Fatal("fresh intent did not match its wire payload")
	}
	dir := filepath.Join(transcript.CodexHome(), "sessions", "2026", "10", "03")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": "worker-session", "originator": originator, "cwd": target.Cwd}})
	prompt, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": wire}}}})
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-03T00-00-00-worker-session.jsonl"), append(append(meta, '\n'), append(prompt, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	return target, wire, intent, now
}

func TestCodexBindingExactPayloadAndOneUse(t *testing.T) {
	target, wire, intent, now := bindingFixture(t, "codex-tui")
	second, err := PrepareCodexBinding(target, "Repair the test issue", now)
	if err != nil || second == wire {
		t.Fatal("identical tasks must get different witnesses", err)
	}
	b, err := os.ReadFile(codexBindingPath(intent.Token))
	if err != nil || strings.Contains(string(b), "Repair the test issue") || len(intent.SHA) != 64 {
		t.Fatal("intent must store a full digest, not prompt text", err)
	}
	for _, altered := range []string{"tampered\n\n" + transcript.SessionBindingPrefix + intent.Token, wire + "\n", wire[:len(wire)-1], transcript.SessionBindingPrefix + "../../no"} {
		if _, ok := CodexBindingForPrompt(altered, now); ok {
			t.Fatal("partial or altered delivery acquired an owner")
		}
	}
	if got, ok := CodexBindingForSession("worker-session", now); !ok || got != intent {
		t.Fatal("submitted rollout did not recover its intent")
	}
	if err := CompleteCodexBinding(intent, target, "worker-session", now); err != nil {
		t.Fatal(err)
	}
	rec, ok := Load(target.Loc)
	if !ok || rec.SessionID != "worker-session" || rec.Cwd != target.Cwd {
		t.Fatal("successful witness did not bind the target", rec)
	}
	if _, ok := CodexBindingForPrompt(wire, now); ok {
		t.Fatal("consumed witness remained available")
	}
	if err := CompleteCodexBinding(intent, target, "worker-session", now); err == nil {
		t.Fatal("consumed intent could be replayed")
	}
}

func TestCodexBindingRejectsChangedTargets(t *testing.T) {
	for _, kind := range []string{"pane", "loc", "pid", "pane pid", "cwd", "owner", "missing log", "desktop", "unknown", "expired", "future", "changed intent", "concurrent consumer"} {
		t.Run(kind, func(t *testing.T) {
			origin := "codex-tui"
			if kind == "desktop" {
				origin = "Codex Desktop"
			} else if kind == "unknown" {
				origin = "unidentified"
			}
			target, wire, intent, now := bindingFixture(t, origin)
			live, sid, at := target, "worker-session", now
			switch kind {
			case "pane":
				live.Pane = "%28"
			case "loc":
				live.Loc = "another:0.0"
			case "pid":
				live.PID++
			case "pane pid":
				live.PanePID++
			case "cwd":
				live.Cwd = t.TempDir()
			case "owner":
				if err := Save(target.Loc, Record{Agent: "codex", SessionID: "newer-owner"}); err != nil {
					t.Fatal(err)
				}
			case "missing log":
				sid = "missing-session"
			case "expired":
				at += int64(codexBindingTTL/time.Second) + 1
			case "future":
				at--
			case "changed intent":
				intent.SHA = strings.Repeat("0", 64)
			case "concurrent consumer":
				if err := os.WriteFile(fileFor(target.Loc)+".binding-lock", nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := CompleteCodexBinding(intent, live, sid, at); err == nil {
				t.Fatal("unsafe witness was accepted")
			}
			if rec, ok := Load(target.Loc); ok && (kind != "owner" || rec.SessionID != "newer-owner") {
				t.Fatal("rejected witness changed resume ownership", rec)
			}
			if _, ok := CodexBindingForPrompt(wire, now); !ok {
				t.Fatal("failure consumed the intent")
			}
		})
	}
}

func TestCodexBindingFailedWriteCanRetry(t *testing.T) {
	target, wire, intent, now := bindingFixture(t, "codex-tui")
	// A directory at the record path forces Save to fail without depending on
	// permission bits (which would pass under a privileged test runner).
	if err := os.Mkdir(fileFor(target.Loc), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CompleteCodexBinding(intent, target, "worker-session", now); err == nil {
		t.Fatal("write failure was reported as success")
	}
	if _, ok := CodexBindingForPrompt(wire, now); !ok {
		t.Fatal("failed Save lost retry evidence")
	}
	if err := os.Remove(fileFor(target.Loc)); err != nil {
		t.Fatal(err)
	}
	if err := CompleteCodexBinding(intent, target, "worker-session", now); err != nil {
		t.Fatal("retry failed", err)
	}
}

func TestCodexBindingExpiryPrunesOnlyExpiredIntents(t *testing.T) {
	_, wire, intent, now := bindingFixture(t, "codex-tui")
	if _, ok := CodexBindingForPrompt(wire, now-1); ok {
		t.Fatal("future intent accepted")
	}
	if _, ok := CodexBindingForSession("worker-session", now+601); ok {
		t.Fatal("expired intent accepted")
	}
	if _, err := os.Stat(codexBindingPath(intent.Token)); !os.IsNotExist(err) {
		t.Fatal("expired generated intent was not pruned", err)
	}
}

func TestCodexClientIdentityFollowsProcessNotShell(t *testing.T) {
	for _, tc := range []struct {
		name, snapshot string
		want           int
	}{
		{"client beneath shell", "58325 1 bash\n58990 58325 /opt/codex\n60000 58990 codex", 58990},
		{"new client same shell", "58325 1 bash\n58991 58325 codex", 58991},
		{"client execed in pane", "58325 1 codex", 58325},
		{"wrapper", "58325 1 bash\n58800 58325 node\n58990 58800 codex", 58990},
		{"unrelated client", "58325 1 bash\n58990 1 codex", 0},
		{"ambiguous branches", "58325 1 bash\n58990 58325 codex\n58991 58325 codex", 0},
		{"missing client", "58325 1 bash\n58990 58325 git", 0},
		{"malformed cycle", "58325 58326 bash\n58326 58325 bash", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexClientPID(58325, tc.snapshot); got != tc.want {
				t.Fatalf("client identity = %d, want %d", got, tc.want)
			}
		})
	}
}
