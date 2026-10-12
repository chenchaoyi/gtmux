package native

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func retirementFixture(t *testing.T) (Binding, Record) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	b := Binding{ID: "binding-a", SessionID: "thread-a", Pane: "%7", Loc: "worker:0.0", Cwd: "/test/repo",
		PID: 123, PanePID: 122, PreparedAt: 1000, BoundAt: 1001}
	r := Record{Agent: "codex", SessionID: b.SessionID, Cwd: b.Cwd, State: "idle", UpdatedAt: 1000, Event: "SessionStart"}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	r, _ = Load(r.SessionID)
	return b, r
}

func TestBindingRetiresExactProvisionalObservation(t *testing.T) {
	b, r := retirementFixture(t)
	if err := ConfirmBinding(b, r); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(r.SessionID); ok {
		t.Fatal("provisional record survived exact binding")
	}
	v, ok := BindingFor(r.SessionID)
	if !ok || !v.Retired || v.Observation != r || v.Pane != b.Pane || v.ID != b.ID {
		t.Fatal("missing exact receipt", v)
	}
	if err := ConfirmBinding(b, r); err != nil {
		t.Fatal("idempotent retry failed", err)
	}
	for _, path := range []string{bindingPath(r.SessionID), filepath.Join(Dir(), ".lock")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("private persistence", path, info, err)
		}
	}
}

func TestBindingPreservesRealAndUnprovenNativeObservations(t *testing.T) {
	for _, kind := range []string{"process", "terminal", "turn", "working", "stop", "newer", "too old", "other cwd", "other agent", "legacy startup", "delayed startup"} {
		t.Run(kind, func(t *testing.T) {
			b, r := retirementFixture(t)
			switch kind {
			case "process":
				r.PID = 123
			case "terminal":
				r.Terminal = "Ghostty"
			case "turn":
				r.TurnID = "turn-a"
			case "working":
				r.State = "working"
			case "stop":
				r.Event = "Stop"
			case "newer":
				r.UpdatedAt = 1002
			case "too old":
				r.UpdatedAt = 399
			case "other cwd":
				r.Cwd = "/another/repo"
			case "other agent":
				r.Agent = "claude"
			case "legacy startup":
				r.Event = ""
			case "delayed startup":
				r.UpdatedAt = 1001
			}
			if err := Save(r); err != nil {
				t.Fatal(err)
			}
			r, _ = Load(r.SessionID)
			if kind == "legacy startup" {
				r.Revision = ""
				data, _ := json.Marshal(r)
				if err := os.WriteFile(fileFor(r.SessionID), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := ConfirmBinding(b, r); err != nil {
				t.Fatal(err)
			}
			_, ok := Load(r.SessionID)
			if ok != (kind != "legacy startup" && kind != "delayed startup") {
				t.Fatalf("%s record retained=%v", kind, ok)
			}
		})
	}
}

func TestBindingCannotReplaceAnotherSameSecondBinding(t *testing.T) {
	b, r := retirementFixture(t)
	if err := ConfirmBinding(b, r); err != nil {
		t.Fatal(err)
	}
	other := b
	other.ID = "binding-b"
	other.PID++
	if err := ConfirmBinding(other, r); err == nil {
		t.Fatal("same-second delayed owner overwrote binding")
	}
	got, _ := BindingFor(r.SessionID)
	if got.ID != b.ID || got.PID != b.PID {
		t.Fatal("original proof lost", got)
	}
}

func TestBindingCannotDeleteSameSecondReplacement(t *testing.T) {
	b, old := retirementFixture(t)
	// Another process writes the same fields in the same second. Only its generation
	// distinguishes this new attachment; timestamp/content-only CAS would delete it.
	c := exec.Command(os.Args[0], "-test.run=^TestNativeGenerationChild$")
	c.Env = append(os.Environ(), "GTMUX_NATIVE_CHILD=1", "GTMUX_NATIVE_CHILD_HOME="+os.Getenv("HOME"))
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	newer, ok := Load(old.SessionID)
	if !ok || newer.Revision == old.Revision || newer.UpdatedAt != old.UpdatedAt {
		t.Fatal("not a same-second replacement", newer)
	}
	if err := ConfirmBinding(b, old); err != nil {
		t.Fatal(err)
	}
	if got, ok := Load(old.SessionID); !ok || got != newer {
		t.Fatal("new attachment lost", got)
	}
	if removed, err := removeIfUnchanged(old); err != nil || removed {
		t.Fatal("stale liveness prune removed new attachment", removed, err)
	}
}

func TestNativeGenerationChild(t *testing.T) {
	if os.Getenv("GTMUX_NATIVE_CHILD") != "1" {
		t.Skip("isolated child only")
	}
	home := os.Getenv("GTMUX_NATIVE_CHILD_HOME")
	if home == "" || !strings.HasPrefix(home, filepath.Clean(os.TempDir())+string(os.PathSeparator)) {
		t.Fatal("child home must be an isolated temporary fixture")
	}
	t.Setenv("HOME", home)
	r, ok := Load("thread-a")
	if !ok {
		t.Fatal("missing parent fixture")
	}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
}

func TestBindingWriteFailurePreservesRecordAndRetries(t *testing.T) {
	b, r := retirementFixture(t)
	if err := os.WriteFile(bindingsDir(), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmBinding(b, r); err == nil {
		t.Fatal("receipt failure reported success")
	}
	if got, ok := Load(r.SessionID); !ok || got != r {
		t.Fatal("observation lost on write failure", got)
	}
	if err := os.Remove(bindingsDir()); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmBinding(b, r); err != nil {
		t.Fatal("retry failed", err)
	}
	if _, ok := Load(r.SessionID); ok {
		t.Fatal("retry did not retire")
	}
}

func TestCorruptBindingCannotBeSilentlyOverwritten(t *testing.T) {
	b, r := retirementFixture(t)
	if err := os.MkdirAll(bindingsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"id":"old","session_id":"another-thread"}`)
	if err := os.WriteFile(bindingPath(r.SessionID), bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmBinding(b, r); err == nil {
		t.Fatal("corrupt receipt treated as missing")
	}
	if got, ok := Load(r.SessionID); !ok || got != r {
		t.Fatal("corrupt proof removed observation", got)
	}
	if bytes, err := os.ReadFile(bindingPath(r.SessionID)); err != nil || string(bytes) != string(bad) {
		t.Fatal("corrupt receipt overwritten", err)
	}
}

func TestNativeIdentityMismatchCannotAliasAnotherFile(t *testing.T) {
	_, r := retirementFixture(t)
	bytes, _ := json.Marshal(r)
	if err := os.WriteFile(fileFor("another-thread"), bytes, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load("another-thread"); ok {
		t.Fatal("wrong file claimed another thread")
	}
	rows := Live(1001)
	if len(rows) != 1 || rows[0].SessionID != r.SessionID {
		t.Fatal("identity mismatch counted as a second attachment", rows)
	}
}

func TestLiveReconcilesInterruptedRetirementButNotFreshActivity(t *testing.T) {
	b, r := retirementFixture(t)
	b.Retired, b.Observation = true, r
	if err := os.MkdirAll(bindingsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(bindingPath(r.SessionID), b); err != nil {
		t.Fatal(err)
	}
	if rows := Live(1002); len(rows) != 0 {
		t.Fatal("interrupted old retirement resurfaced", rows)
	}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	if rows := Live(1002); len(rows) != 1 {
		t.Fatal("fresh activity swallowed by retirement", rows)
	}
}

func TestNativeSaveIsAtomicAndBoundedUnderStoreLock(t *testing.T) {
	_, r := retirementFixture(t)
	lock, err := os.OpenFile(filepath.Join(Dir(), ".lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = Save(r)
	if err == nil || !strings.Contains(err.Error(), "timeout") || time.Since(started) > 3*time.Second {
		t.Fatal("lock wait must fail bounded", err, time.Since(started))
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if got, ok := Load(r.SessionID); !ok || got != r {
		t.Fatal("blocked write corrupted record", got)
	}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(fileFor(r.SessionID))
	var got Record
	if err != nil || json.Unmarshal(b, &got) != nil || got.Revision == "" {
		t.Fatal("invalid publication", err)
	}
}

func TestBindingReceiptRetentionIsBounded(t *testing.T) {
	b, r := retirementFixture(t)
	if err := ConfirmBinding(b, r); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < bindingLimit+2; i++ {
		copy := b
		// Use short IDs so the filesystem budget is independent of the count.
		copy.SessionID = time.Unix(int64(i), 0).Format("150405")
		if err := atomicJSON(bindingPath(copy.SessionID), copy); err != nil {
			t.Fatal(err)
		}
	}
	if err := withStoreLock(func() error { pruneBindings(); return nil }); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(bindingsDir())
	if err != nil || len(entries) != bindingLimit {
		t.Fatal("receipt bound", len(entries), err)
	}
}
