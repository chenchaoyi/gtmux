package state

import (
	"os"
	"testing"
	"time"
)

// `gtmux restore` had no guard at all: two runs in parallel each opened the whole working
// set, so a user who clicked the menu bar's restore row twice — what a person does when a
// slow action gives no feedback — got every session and tab twice.
func TestRunLockRefusesASecondRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, b := NewRunLock("restore"), NewRunLock("restore")

	held, _ := a.Acquire(time.Minute)
	if !held {
		t.Fatal("the first caller must get the lock")
	}
	held2, age := b.Acquire(time.Minute)
	if held2 {
		t.Error("a second run took the lock — this is the double restore")
	}
	if age < 0 || age > time.Minute {
		t.Errorf("held-for = %v, want the holder's age", age)
	}

	a.Release()
	if held3, _ := b.Acquire(time.Minute); !held3 {
		t.Error("the lock must be free once released")
	}
}

// A crash must not wedge the feature. Two ways out, and both matter: the owner's process
// is gone, or the lock has run past any plausible duration.
func TestRunLockTakesOverAStaleLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	l := NewRunLock("restore")
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}

	// A lock owned by a pid that cannot be running.
	if err := os.WriteFile(l.path, []byte("999999\n"+itoa(time.Now().Unix())+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if held, _ := l.Acquire(time.Hour); !held {
		t.Error("a lock whose owner is gone must be taken over")
	}

	// A lock owned by THIS process (alive) but older than the max age.
	old := time.Now().Add(-2 * time.Hour).Unix()
	if err := os.WriteFile(l.path, []byte(itoa(int64(os.Getpid()))+"\n"+itoa(old)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if held, _ := l.Acquire(time.Hour); !held {
		t.Error("a lock past its max age must be taken over, even if the pid is alive")
	}
}

// A corrupt or empty lock file must not block the job either — the guard is a courtesy,
// never a gate that can fail closed on garbage.
func TestRunLockIgnoresAGarbageFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	l := NewRunLock("restore")
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.path, []byte("not a lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	if held, _ := l.Acquire(time.Minute); !held {
		t.Error("a garbage lock file must not block the job")
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
