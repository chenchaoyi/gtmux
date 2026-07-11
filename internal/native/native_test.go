package native

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSaveLoadRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := Record{Agent: "claude", SessionID: "sid-1", Cwd: "/x", State: "working", UpdatedAt: 100}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	got, ok := Load("sid-1")
	if !ok || got.State != "working" || got.Agent != "claude" {
		t.Fatalf("Load = %+v, %v", got, ok)
	}
	Remove("sid-1")
	if _, ok := Load("sid-1"); ok {
		t.Error("record should be gone after Remove")
	}
}

// Live returns fresh records and self-prunes stale ones (past StaleAfter).
func TestLivePrunesStale(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_000_000)
	_ = Save(Record{SessionID: "fresh", Agent: "claude", State: "idle", UpdatedAt: now - 60})
	_ = Save(Record{SessionID: "stale", Agent: "claude", State: "idle", UpdatedAt: now - int64(StaleAfter/time.Second) - 1})

	live := Live(now)
	if len(live) != 1 || live[0].SessionID != "fresh" {
		t.Fatalf("Live = %+v, want just the fresh record", live)
	}
	if _, ok := Load("stale"); ok {
		t.Error("stale record should be pruned from disk by Live")
	}
}

// Live prunes a record whose process has EXITED (the phantom "elsewhere" bug) even
// when the record is otherwise fresh; a record for a live process is kept.
func TestLivePrunesDeadProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_000_000)

	// A reaped child → its pid is dead (kill → ESRCH; if the OS reused it, its comm
	// won't be "claude" → still pruned).
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Skipf("cannot run a throwaway process: %v", err)
	}
	deadPID := c.Process.Pid

	_ = Save(Record{SessionID: "dead", Agent: "claude", State: "working", UpdatedAt: now, PID: deadPID, Comm: "claude"})
	_ = Save(Record{SessionID: "alive", Agent: "claude", State: "idle", UpdatedAt: now, PID: os.Getpid()}) // no comm → kept

	live := Live(now)
	if len(live) != 1 || live[0].SessionID != "alive" {
		t.Fatalf("Live = %+v, want just the alive record", live)
	}
	if _, ok := Load("dead"); ok {
		t.Error("record for a dead process should be pruned by Live")
	}
}

// A pid that's alive but whose command no longer matches the recorded comm (pid
// reuse — common right after a reboot) is treated as gone and pruned.
func TestLivePrunesReusedPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_000_000)
	_ = Save(Record{SessionID: "reused", Agent: "claude", State: "idle", UpdatedAt: now,
		PID: os.Getpid(), Comm: "definitely-not-this-process"})
	if live := Live(now); len(live) != 0 {
		t.Fatalf("Live = %+v, want empty (pid reused → comm mismatch → pruned)", live)
	}
}

// A session id with filesystem-hostile chars round-trips (base64url keying).
func TestSaveUnsafeSessionID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sid := "a/b:c d"
	if err := Save(Record{SessionID: sid, Agent: "codex", State: "waiting", UpdatedAt: 5}); err != nil {
		t.Fatal(err)
	}
	if got, ok := Load(sid); !ok || got.State != "waiting" {
		t.Fatalf("unsafe id round-trip failed: %+v %v", got, ok)
	}
}
