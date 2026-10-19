package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func putJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func attachmentFixture(t *testing.T) (string, Attachment, Inspector) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Unix(1900000000, 0)
	a := Attachment{Version: 1, Agent: "dsh", Client: "dst", Pane: "%44", PID: 99,
		Instance: "0123456789abcdef0123456789abcdef", HomeID: "home", SessionID: "session", Cwd: "/work",
		ServerEpoch: "7", BindingEpoch: 9, StartedAt: now.Add(-time.Minute).UnixMilli(), UpdatedAt: now.UnixMilli(), InputReady: true}
	dir := filepath.Join(home, "bridges")
	putJSON(t, filepath.Join(dir, "99.json"), a)
	putJSON(t, filepath.Join(home, ".dsh-tui", "inject", "servers.json"), []any{map[string]any{"pid": 99, "sessionId": "session", "startedAt": 9}})
	putJSON(t, filepath.Join(home, ".dsh-tui", "session-mounts.json"), map[string]any{"version": 1, "owners": []any{map[string]any{"pid": 99, "sessionIds": []string{"session", "background"}}}})
	inspect := Inspector{Now: now, Process: func(pid int) (int, string, error) {
		switch pid {
		case 99:
			return 88, "/node /profile/node_modules/@deepseek-harness-tui/dsh-tui/lib/types/dsh-adapter/host-entry.js", nil
		case 88:
			return 77, "/node /launcher.js", nil
		}
		return 0, "", errors.New("gone")
	}, Pane: func(pane string) (int, string, error) {
		if pane != "%44" {
			return 0, "", errors.New("wrong pane")
		}
		return 77, "7", nil
	}, Session: func(agent, home, id string) bool { return agent == "dsh" && home == "home" && id == "session" }}
	return dir, a, inspect
}

func TestVerifiedForegroundAttachment(t *testing.T) {
	dir, want, inspect := attachmentFixture(t)
	got := Load(dir, inspect)
	if len(got) != 1 || !Same(got[0], want) || !got[0].InputReady {
		t.Fatalf("verified foreground: %+v", got)
	}
}

func TestAttachmentRejectsStaleForeignAndConflictingEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Attachment, *Inspector)
	}{
		{"stale", func(a *Attachment, _ *Inspector) { a.UpdatedAt -= 6000 }},
		{"future", func(a *Attachment, _ *Inspector) { a.UpdatedAt += 6000 }},
		{"wrong home", func(a *Attachment, _ *Inspector) { a.HomeID = "other" }},
		{"wrong session", func(a *Attachment, _ *Inspector) { a.SessionID = "background" }},
		{"switched binding", func(a *Attachment, _ *Inspector) { a.BindingEpoch++ }},
		{"reused tmux server", func(a *Attachment, _ *Inspector) { a.ServerEpoch = "old" }},
		{"web process", func(_ *Attachment, i *Inspector) {
			i.Process = func(int) (int, string, error) { return 77, "node dsh web", nil }
		}},
		{"foreign ancestry", func(_ *Attachment, i *Inspector) {
			i.Process = func(int) (int, string, error) {
				return 1, "/node /profile/node_modules/@deepseek-harness-tui/dsh-tui/lib/types/dsh-adapter/host-entry.js", nil
			}
		}},
		{"unknown client", func(a *Attachment, _ *Inspector) { a.Client = "other" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, a, inspect := attachmentFixture(t)
			test.edit(&a, &inspect)
			putJSON(t, filepath.Join(dir, "99.json"), a)
			if got := Load(dir, inspect); len(got) != 0 {
				t.Fatalf("unverified attachment accepted: %+v", got)
			}
		})
	}
	t.Run("two witnesses", func(t *testing.T) {
		dir, a, inspect := attachmentFixture(t)
		putJSON(t, filepath.Join(dir, "duplicate.json"), a)
		if len(Load(dir, inspect)) != 0 {
			t.Fatal("conflicting witnesses accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		dir, _, inspect := attachmentFixture(t)
		original := filepath.Join(dir, "99.json")
		if err := os.Rename(original, original+".elsewhere"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(original+".elsewhere", original); err != nil {
			t.Fatal(err)
		}
		if len(Load(dir, inspect)) != 0 {
			t.Fatal("linked witness accepted")
		}
	})
}

func TestDSTOtherBackendCannotBorrowBareDSHIdentity(t *testing.T) {
	dir, _, inspect := attachmentFixture(t)
	putJSON(t, filepath.Join(os.Getenv("HOME"), ".dsh-tui", "session-mounts.json"), map[string]any{"version": 1, "owners": []any{map[string]any{"pid": 99, "sessionIds": []string{"session", "codex:session"}}}})
	if len(Load(dir, inspect)) != 0 {
		t.Fatal("ambiguous backend identity accepted")
	}
}

func TestDirectPaneProcessIsValidAncestry(t *testing.T) {
	dir, _, inspect := attachmentFixture(t)
	inspect.Pane = func(string) (int, string, error) { return 99, "7", nil }
	if len(Load(dir, inspect)) != 1 {
		t.Fatal("native host exec'd directly in pane was rejected")
	}
}
