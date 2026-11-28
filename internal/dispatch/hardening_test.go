package dispatch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCHH002_ConcurrentTaskUpdatesSerializeCleanly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	task := Task{
		ID:        "task-atomic-1",
		Pane:      "%9",
		Session:   "work",
		Agent:     "codex",
		Goal:      "verify atomic task persistence",
		CreatedAt: 1000,
	}
	if err := AddTask(task); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	taskFile := filepath.Join(state.StorePath("tasks"), task.ID+".json")
	info, err := os.Lstat(taskFile)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("task file perm = %v err=%v, want 0600", info.Mode().Perm(), err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			SetDisposition(task.ID, DispositionAwaitingCommander, int64(2000+i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			_ = SnoozeTask(task.ID, int64(5000+i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			MarkDelivered("%9", "verify atomic task persistence now", int64(3000+i))
		}
	}()
	wg.Wait()

	got, ok := TaskForPane("%9")
	if !ok {
		t.Fatal("TaskForPane returned false after concurrent updates")
	}
	if !got.Delivered || got.Disposition != DispositionAwaitingCommander || got.SnoozeUntil < 5000 {
		t.Fatalf("concurrent task updates lost fields: %+v", got)
	}
}
