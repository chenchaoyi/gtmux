package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cacheHistory(t *testing.T, dir string, n int, response string) string {
	t.Helper()
	path := filepath.Join(dir, fmt.Sprintf("session-%d.jsonl", n))
	body := chatRecord("user_message", fmt.Sprintf("question %d", n)) + chatRecord("agent_message", response)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTranscriptCacheEvictsSessionsAndReadsThemAgain(t *testing.T) {
	l := &Loader{}
	dir := t.TempDir()
	first := ""
	for i := 0; i < 20; i++ {
		path := cacheHistory(t, dir, i, "answer")
		if i == 0 {
			first = path
		}
		if _, err := l.loadCodexFile(path, 300); err != nil {
			t.Fatal(err)
		}
	}
	if len(l.m) > 16 {
		t.Fatalf("cache retained %d sessions; limit is 16", len(l.m))
	}
	if _, ok := l.m[first]; ok {
		t.Fatal("least recently used history stayed pinned")
	}
	turns, err := l.loadCodexFile(first, 300)
	if err != nil || len(turns) != 1 || turns[0].Prompt != "question 0" || turns[0].Response != "answer" {
		t.Fatalf("evicted history could not be read again: turns=%#v err=%v", turns, err)
	}
	f, err := os.OpenFile(first, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(chatRecord("user_message", "next") + chatRecord("agent_message", "new answer"))
	if closeErr := f.Close(); err != nil || closeErr != nil {
		t.Fatalf("append: %v / %v", err, closeErr)
	}
	turns, err = l.loadCodexFile(first, 300)
	if err != nil || len(turns) != 2 || turns[1].Response != "new answer" {
		t.Fatalf("append after eviction: turns=%#v err=%v", turns, err)
	}
}

func TestTranscriptCacheHasByteBudget(t *testing.T) {
	l := &Loader{}
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		path := cacheHistory(t, dir, i, strings.Repeat("a", 1<<20))
		if _, err := l.loadCodexFile(path, 300); err != nil {
			t.Fatal(err)
		}
	}
	retained := 0
	for _, e := range l.m {
		body, err := json.Marshal(e.turns)
		if err != nil {
			t.Fatal(err)
		}
		retained += len(body)
	}
	if retained > 32<<20 {
		t.Fatalf("cache retained %d transcript bytes; budget is 32 MiB", retained)
	}
}

func TestTranscriptCacheKeepsRecentlyUsedAndExpiresIdle(t *testing.T) {
	now := time.Unix(1000, 0)
	l := &Loader{cacheNow: func() time.Time { return now }}
	dir := t.TempDir()
	paths := make([]string, 16)
	for i := range paths {
		paths[i] = cacheHistory(t, dir, i, "answer")
		if _, err := l.loadCodexFile(paths[i], 1); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	if _, err := l.loadCodexFile(paths[0], 1); err != nil {
		t.Fatal(err)
	}
	extra := cacheHistory(t, dir, 16, "answer")
	if _, err := l.loadCodexFile(extra, 1); err != nil {
		t.Fatal(err)
	}
	if l.m[paths[0]] == nil || l.m[paths[1]] != nil {
		t.Fatal("cache did not retain the history actually read most recently")
	}
	now = now.Add(25 * time.Hour)
	turns, err := l.loadCodexFile(extra, 1)
	if err != nil || len(turns) != 1 || turns[0].Response != "answer" {
		t.Fatalf("idle re-read: %#v %v", turns, err)
	}
	if len(l.m) != 1 {
		t.Fatalf("idle histories retained: %d", len(l.m))
	}
}
