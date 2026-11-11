package transcript

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLCL001LargeCodexTailStaysCachedAndEarlierHistoryRemainsReadable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	id := "large-cache-fixture"
	dir := filepath.Join(home, "sessions", "2026", "11", "11")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-11-11T00-00-00-"+id+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fmt.Fprintf(f, "{\"type\":\"session_meta\",\"payload\":{\"id\":%q,\"originator\":\"codex-tui\"}}\n", id); err != nil {
		t.Fatal(err)
	}
	response := strings.Repeat("synthetic paragraph ", (256<<10)/20) + "end"
	const total = 72
	for i := 1; i <= total; i++ {
		if _, err = f.WriteString(chatRecord("user_message", fmt.Sprintf("question %d", i)) + chatRecord("agent_message", response)); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	first, err := l.loadCodexPaths([]string{path}, 300)
	if err != nil || len(first) == 0 || first[len(first)-1].Ordinal != total || first[len(first)-1].Response != response {
		t.Fatalf("latest content/ordinal changed: turns=%d err=%v", len(first), err)
	}
	entry := l.m[path]
	if entry == nil {
		t.Fatal("large session discarded its entire reusable cache")
	}
	if entry.retainedBytes > cacheByteCap {
		t.Fatalf("retained %d bytes above %d", entry.retainedBytes, cacheByteCap)
	}
	warm, err := l.loadCodexPaths([]string{path}, 20)
	if err != nil || l.m[path] != entry || warm[len(warm)-1].RecordID != first[len(first)-1].RecordID || warm[len(warm)-1].Response != response {
		t.Fatal("unchanged source reparsed or latest content changed", err)
	}
	full, err := l.loadCodexPaths([]string{path}, 300)
	if err != nil || len(full) != total || full[0].Ordinal != 1 || full[total-1].Response != response {
		t.Fatal("cache coverage silently truncated a full request", err)
	}
	SetHistoryCursor("codex", id, warm)
	cursor := warm[0].HistoryBefore
	if warm[0].Ordinal <= 1 || cursor == "" {
		t.Fatal("bounded tail did not expose its omitted prefix")
	}
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(chatRecord("user_message", "question 73") + chatRecord("agent_message", "new answer")); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := l.loadCodexPaths([]string{path}, 20)
	if err != nil || len(next) == 0 || next[len(next)-1].Ordinal != total+1 || next[len(next)-1].Response != "new answer" || l.m[path] == nil || l.m[path].retainedBytes > cacheByteCap {
		t.Fatal("append lost cache, content or original ordinal", err)
	}
	if warm[len(warm)-1].Ordinal != total || warm[len(warm)-1].Response != response {
		t.Fatal("append mutated a previously returned snapshot")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := warm[0].Ordinal - 1
	for pages := 0; cursor != ""; pages++ {
		if pages >= total {
			t.Fatal("history pagination did not progress")
		}
		page, err := LoadBefore(context.Background(), "codex", id, cursor)
		if err != nil || len(page) == 0 || page[len(page)-1].Ordinal != want {
			t.Fatalf("earlier source history changed: want=%d count=%d err=%v", want, len(page), err)
		}
		for _, turn := range page {
			if turn.Prompt != fmt.Sprintf("question %d", turn.Ordinal) || turn.Response != response {
				t.Fatal("earlier full public content was truncated")
			}
		}
		want = page[0].Ordinal - 1
		SetHistoryCursor("codex", id, page)
		cursor = page[0].HistoryBefore
	}
	if want != 0 {
		t.Fatal("earlier history ended before the first turn")
	}
	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("history read modified source bytes", err)
	}
}

func TestLCL001IndividuallyOversizedTurnIsCompleteAndUncached(t *testing.T) {
	response := strings.Repeat("z", 17<<20)
	path := cacheHistory(t, t.TempDir(), 1, response)
	l := &Loader{}
	turns, err := l.loadCodexFile(path, 20)
	if err != nil || len(turns) != 1 || turns[0].Response != response {
		t.Fatal("individual turn was truncated", len(turns), err)
	}
	if e := l.m[path]; e != nil && e.retainedBytes > cacheByteCap {
		t.Fatal("oversized turn bypassed cache budget")
	}
}
