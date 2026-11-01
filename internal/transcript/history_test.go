package transcript

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func historyFixture(t *testing.T, n int) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "11", "01")
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	id := "test-history"
	path := filepath.Join(dir, "rollout-2026-11-01T00-00-00-"+id+".jsonl")
	var b strings.Builder
	b.WriteString(`{"type":"session_meta","payload":{"id":"test-history","originator":"codex-tui"}}` + "\n")
	for i := 1; i <= n; i++ {
		b.WriteString(chatRecord("user_message", fmt.Sprintf("question %d", i)))
		b.WriteString(chatRecord("agent_message", fmt.Sprintf("answer %d", i)))
	}
	if e := os.WriteFile(path, []byte(b.String()), 0600); e != nil {
		t.Fatal(e)
	}
	return id, path
}
func TestHistoryPagesBeyondCacheAndAppend(t *testing.T) {
	id, path := historyFixture(t, 1050)
	l := &Loader{}
	latest, e := l.loadCodexPaths([]string{path}, 10)
	if e != nil || len(latest) != 10 || latest[0].Ordinal != 1041 || latest[9].Ordinal != 1050 {
		t.Fatalf("tail: %v %v", latest, e)
	}
	SetHistoryCursor("codex", id, latest)
	cursor := latest[0].HistoryBefore
	if cursor == "" || strings.Contains(cursor, path) {
		t.Fatal("cursor missing or exposes path")
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString(chatRecord("user_message", "question 1051"))
	_ = f.Close()
	next, e := l.loadCodexPaths([]string{path}, 10)
	if e != nil || next[9].Ordinal != 1051 {
		t.Fatal("append ordinal", e, next)
	}
	want := 1040
	for cursor != "" {
		page, e := LoadBefore(context.Background(), "codex", id, cursor)
		if e != nil || len(page) == 0 || len(page) > 50 || page[len(page)-1].Ordinal != want {
			t.Fatalf("page end %d len%d err%v", want, len(page), e)
		}
		for i := 1; i < len(page); i++ {
			if page[i].Ordinal != page[i-1].Ordinal+1 {
				t.Fatal("page gap")
			}
		}
		want = page[0].Ordinal - 1
		SetHistoryCursor("codex", id, page)
		cursor = page[0].HistoryBefore
	}
	if want != 0 {
		t.Fatal("history ended early", want)
	}
}
func TestHistoryRejectsWrongSessionReplacementAndCanceledRead(t *testing.T) {
	id, path := historyFixture(t, 167)
	l := &Loader{}
	latest, _ := l.loadCodexPaths([]string{path}, 10)
	SetHistoryCursor("codex", id, latest)
	if latest[0].Ordinal != 158 {
		t.Fatal("must retain original 158..167 numbering")
	}
	cursor := latest[0].HistoryBefore
	if _, e := LoadBefore(context.Background(), "codex", "other", cursor); !errors.Is(e, ErrHistoryChanged) {
		t.Fatal("cross-session cursor accepted", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := LoadBefore(ctx, "codex", id, cursor); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel", e)
	}
	b, _ := os.ReadFile(path)
	replacement := path + ".replacement"
	_ = os.WriteFile(replacement, b, 0600)
	_ = os.Rename(replacement, path)
	if _, e := LoadBefore(context.Background(), "codex", id, cursor); !errors.Is(e, ErrHistoryChanged) {
		t.Fatal("replacement accepted", e)
	}
}
func TestHistoryOrdinalAcrossRolloutsAndGenericCache(t *testing.T) {
	id, first := historyFixture(t, 810)
	second := filepath.Join(t.TempDir(), "second.jsonl")
	_ = os.WriteFile(second, []byte(chatRecord("user_message", "continued")), 0600)
	l := &Loader{}
	ts, e := l.loadCodexPaths([]string{first, second}, 5)
	if e != nil || ts[0].Ordinal != 807 || ts[4].Ordinal != 811 {
		t.Fatal("rollout offset", id, ts, e)
	}
	// Local cache numbers must remain local when reused in a later combined read.
	local, _ := l.loadCodexFile(second, 5)
	if local[0].Ordinal != 1 {
		t.Fatal("mutated shared cache")
	}
	claude := filepath.Join(t.TempDir(), "claude.jsonl")
	var b strings.Builder
	for i := 0; i < 900; i++ {
		b.WriteString(fmt.Sprintf("{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"q%d\"}}\n", i))
	}
	_ = os.WriteFile(claude, []byte(b.String()), 0600)
	ts, e = l.loadFile(claude, claudeStep, 5)
	if e != nil || ts[0].Ordinal != 896 {
		t.Fatal("generic ordinal", e)
	}
	f, _ := os.OpenFile(claude, os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString("{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"next\"}}\n")
	_ = f.Close()
	ts, e = l.loadFile(claude, claudeStep, 5)
	if e != nil || ts[4].Ordinal != 901 {
		t.Fatal("generic incremental ordinal", e)
	}
}

func TestHistoryOffsetCacheIsBoundedAndReusable(t *testing.T) {
	id, path := historyFixture(t, 1050)
	l := &Loader{}
	ts, _ := l.loadCodexPaths([]string{path}, 10)
	SetHistoryCursor("codex", id, ts)
	page, e := LoadBefore(context.Background(), "codex", id, ts[0].HistoryBefore)
	if e != nil {
		t.Fatal(e)
	}
	fi, _ := os.Stat(path)
	entry, ok := getHistoryIndex(path, fi)
	if !ok || len(entry.bounds) > historyIndexTurns || entry.bounds[0].ordinal != 1 {
		t.Fatal("missing reusable source positions")
	}
	target := page[0].Ordinal - HistoryPageTurns
	position := int64(0)
	for _, b := range entry.bounds {
		if b.ordinal == target {
			position = b.offset
		}
	}
	if position <= 0 {
		t.Fatal("next page would unnecessarily start at the file beginning")
	}
	SetHistoryCursor("codex", id, page)
	next, e := LoadBefore(context.Background(), "codex", id, page[0].HistoryBefore)
	if e != nil || next[0].Ordinal != target || next[len(next)-1].Ordinal != page[0].Ordinal-1 {
		t.Fatal("indexed page", e)
	}
	b, _ := os.ReadFile(path)
	_ = os.WriteFile(path, b, 0600)
	if _, ok := getHistoryIndex(path, mustHistoryStat(t, path)); ok {
		t.Fatal("same-size rewritten source retained stale offsets")
	}
	if _, e := LoadBefore(context.Background(), "codex", id, page[0].HistoryBefore); !errors.Is(e, ErrHistoryChanged) {
		t.Fatal("rewritten source accepted cursor", e)
	}
}
func mustHistoryStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	fi, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	return fi
}
