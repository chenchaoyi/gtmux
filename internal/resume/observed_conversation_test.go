package resume

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func observationFixture(t *testing.T) (native.Binding, codexterminal.Pane, int64, string) {
	t.Helper()
	r, p, now := processBindingFixture(t)
	b := native.Binding{Kind: "frontend", BackendPID: 909, BackendStart: "former shared backend", HomeID: r.HomeID, ID: "0123456789abcdef0123456789abcdef", SessionID: r.SessionID, Pane: p.ID, Loc: p.Loc, Cwd: p.Cwd, PID: 202, PanePID: p.PID, PreparedAt: now - 200, BoundAt: now - 100}
	if err := native.ConfirmBinding(b, r); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(native.Dir(), "bindings", b.HomeID, base64.RawURLEncoding.EncodeToString([]byte(b.SessionID))+".json")
	if err := os.WriteFile(fileFor(p.Loc), []byte(`{"sessionId":"untrusted prefix"} trailing`), 0600); err != nil {
		t.Fatal(err)
	}
	birth := time.Unix(now-400, 0).In(time.Local).Format("Mon Jan 2 15:04:05 2006")
	codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
		return map[string]codexterminal.Client{p.ID: {PID: b.PID, Start: birth, RootStart: birth, TTY: "ttys001"}}
	}
	return b, p, now, path
}

func TestSID003LastKnownHistoryHasNoLiveAuthority(t *testing.T) {
	for _, kind := range []string{"corrupt", "missing", "expired-same"} {
		t.Run(kind, func(t *testing.T) {
			b, p, now, _ := observationFixture(t)
			if kind == "missing" {
				Remove(p.Loc)
			}
			if kind == "expired-same" {
				if err := writeBindingJSON(fileFor(p.Loc), Record{Agent: "codex", HomeID: b.HomeID, SessionID: b.SessionID, Frontend: b.ID, UpdatedAt: now - 1000}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(fileFor(p.Loc))
			oldNative, _ := native.Load(b.SessionID, b.HomeID)
			got := ObservedCodexConversations(transcript.NewCodexPathScope(), map[string]string{p.ID: p.Loc}, now)
			if v := got[p.ID]; v.SessionID != b.SessionID || v.State != "last-known" || v.HomeID != b.HomeID {
				t.Fatalf("no precise historical view: %+v", got)
			}
			if _, ok := LoadLive(p.Loc); ok {
				t.Fatal("history gained current binding")
			}
			if _, ok := CodexFrontendForThread(b.HomeID, b.SessionID); ok {
				t.Fatal("history gained input authority")
			}
			after, _ := os.ReadFile(fileFor(p.Loc))
			newNative, _ := native.Load(b.SessionID, b.HomeID)
			if !bytes.Equal(before, after) || oldNative != newNative {
				t.Fatal("read-only projection changed its sources")
			}
		})
	}
}

func TestSID003UnknownConflictingAndUnsafeHistoryStaysUnassociated(t *testing.T) {
	for _, kind := range []string{"missing receipt", "foreign-home", "other session", "other frontend", "other agent", "ambiguous", "unknown competitor", "wrong pid", "wrong root", "changed pane", "missing tty", "reused pid", "reused root", "same-second birth", "unknown birth", "headless", "future", "invalid instance", "desktop", "subagent", "symlink", "public file", "public dir", "oversize", "trailing json"} {
		t.Run(kind, func(t *testing.T) {
			b, p, now, path := observationFixture(t)
			save := func() {
				raw, _ := json.Marshal(b)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			birth := time.Unix(now-400, 0).In(time.Local).Format("Mon Jan 2 15:04:05 2006")
			c := codexterminal.Client{PID: b.PID, Start: birth, RootStart: birth, TTY: "ttys001"}
			switch kind {
			case "missing receipt":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "foreign-home":
				b.HomeID = "00000000000000000000000000000000"
				save()
			case "other session", "other frontend", "other agent":
				incumbent := Record{Agent: "codex", HomeID: b.HomeID, SessionID: b.SessionID, Frontend: b.ID}
				if kind == "other session" {
					incumbent.SessionID = "other"
				}
				if kind == "other frontend" {
					incumbent.Frontend = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				}
				if kind == "other agent" {
					incumbent.Agent = "claude"
				}
				if err := writeBindingJSON(fileFor(p.Loc), incumbent); err != nil {
					t.Fatal(err)
				}
			case "ambiguous", "unknown competitor":
				other := b
				other.SessionID = "competing-session"
				other.ID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				if kind == "unknown competitor" {
					other.Kind = "unknown"
				}
				if kind == "ambiguous" {
					writeProcessSession(t, other.SessionID, "codex-tui", `"cli"`, p.Cwd)
				}
				if err := native.ConfirmBinding(other, native.Record{}); err != nil {
					t.Fatal(err)
				}
			case "wrong pid":
				c.PID++
			case "wrong root":
				b.PanePID++
				save()
			case "changed pane":
				p.Loc = "moved:0.0"
			case "missing tty":
				c.TTY = ""
			case "reused pid":
				c.Start = time.Unix(now-100, 0).In(time.Local).Format("Mon Jan 2 15:04:05 2006")
			case "reused root":
				c.RootStart = time.Unix(now-100, 0).In(time.Local).Format("Mon Jan 2 15:04:05 2006")
			case "same-second birth":
				c.Start = time.Unix(b.PreparedAt, 0).In(time.Local).Format("Mon Jan 2 15:04:05 2006")
			case "unknown birth":
				c.Start = "unknown"
			case "headless":
				codexProcessInteractive = func(int) bool { return false }
			case "future":
				b.PreparedAt = now + 1
				b.BoundAt = now + 1
				save()
			case "invalid instance":
				b.ID = "unknown"
				save()
			case "desktop":
				writeProcessSession(t, b.SessionID, "Codex Desktop", `"vscode"`, p.Cwd)
			case "subagent":
				writeProcessSession(t, b.SessionID, "codex-tui", `{"subagent":{"parent_thread_id":"parent"}}`, p.Cwd)
			case "symlink":
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".saved", path); err != nil {
					t.Fatal(err)
				}
			case "public file":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "public dir":
				if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				if err := os.WriteFile(path, bytes.Repeat([]byte(" "), 8193), 0600); err != nil {
					t.Fatal(err)
				}
			case "trailing json":
				raw, _ := os.ReadFile(path)
				if err := os.WriteFile(path, append(raw, []byte(" {}")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			codexProcessClients = func([]codexterminal.Pane) map[string]codexterminal.Client {
				return map[string]codexterminal.Client{p.ID: c}
			}
			if got := ObservedCodexConversations(transcript.NewCodexPathScope(), map[string]string{p.ID: p.Loc}, now); len(got) != 0 {
				t.Fatalf("%s acquired historical projection: %+v", kind, got)
			}
		})
	}
}
