package notify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func enableQueue(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	old := desktopNotifications
	desktopNotifications = func() bool { return true }
	t.Cleanup(func() { desktopNotifications = old })
}
func TestQueueBoundsBacklogAndKeepsNewest(t *testing.T) {
	enableQueue(t)
	dir := state.NotifyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < queueLimit+20; i++ {
		name := fmt.Sprintf("%d-p1.json", now.UnixNano()-int64(i+1))
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := fmt.Sprintf(".%d-p1.json.tmp", now.Add(-time.Hour).UnixNano())
	if err := os.WriteFile(filepath.Join(dir, old), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "unrelated-notes.txt")
	if err := os.WriteFile(foreign, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	Send(Options{Kind: "input", Message: "newest", Pane: "%1"})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	found := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
			b, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
			found = found || strings.Contains(string(b), "newest")
		}
	}
	if count > queueLimit || !found {
		t.Fatalf("count=%d newest=%v", count, found)
	}
	if _, err := os.Stat(filepath.Join(dir, old)); !os.IsNotExist(err) {
		t.Fatal("expired orphan remained")
	}
	if b, err := os.ReadFile(foreign); err != nil || string(b) != "keep" {
		t.Fatal("unrelated file changed")
	}
}
func TestConcurrentQueueWritersRemainBounded(t *testing.T) {
	enableQueue(t)
	var wg sync.WaitGroup
	for i := 0; i < queueLimit+30; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Send(Options{Kind: "done", Pane: "%1"}) }()
	}
	wg.Wait()
	entries, err := os.ReadDir(state.NotifyDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > queueLimit {
		t.Fatalf("concurrent queue grew to %d", len(entries))
	}
}
