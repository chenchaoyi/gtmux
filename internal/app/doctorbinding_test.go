package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// writeSession lays down a Claude-shaped log whose LAST message is at `last`, and
// stamps the FILE's mtime independently — the two are different facts, and telling
// them apart is what this row is about.
func writeSession(t *testing.T, dir, id string, last, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, id+".jsonl")
	body := fmt.Sprintf(`{"type":"user","timestamp":%q}`+"\n", last.UTC().Format(time.RFC3339Nano))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

func sessionDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The failure this row exists for: the conversation moved to a session nobody is
// bound to, and it is still going.
func TestNewestUnclaimedSessionFindsTheLiveRival(t *testing.T) {
	dir := sessionDir(t)
	now := time.Now()
	boundLast := now.Add(-5 * time.Hour)
	bound := writeSession(t, dir, "bound", boundLast, boundLast)
	rivalLast := now.Add(-3 * time.Minute)
	writeSession(t, dir, "rival", rivalLast, rivalLast)

	rec := resume.Record{Agent: "claude", SessionID: "bound"}
	got := newestUnclaimedSession(bound, rec, map[string]bool{"bound": true}, boundLast.Unix())
	if got != rivalLast.Unix() {
		t.Fatalf("live rival not found: got %d, want %d", got, rivalLast.Unix())
	}
}

// The exact trap this row fell into on the first cut: the DEAD log had the NEWER
// mtime — Claude appended a `permission-mode` record to it hours after the
// conversation had moved on — so an mtime pre-filter skipped the very candidate it
// existed to find and the row stayed green through the real failure.
func TestNewestUnclaimedSessionIgnoresAMisleadingMtime(t *testing.T) {
	dir := sessionDir(t)
	now := time.Now()
	boundLast := now.Add(-5 * time.Hour)
	// the dead log: last MESSAGE five hours ago, touched one minute ago
	bound := writeSession(t, dir, "bound", boundLast, now.Add(-1*time.Minute))
	rivalLast := now.Add(-3 * time.Minute)
	writeSession(t, dir, "rival", rivalLast, rivalLast)

	rec := resume.Record{Agent: "claude", SessionID: "bound"}
	got := newestUnclaimedSession(bound, rec, map[string]bool{"bound": true}, boundLast.Unix())
	if got != rivalLast.Unix() {
		t.Fatalf("a newer mtime on the dead log hid the live rival: got %d, want %d", got, rivalLast.Unix())
	}
}

// A session another pane owns is that pane's business — two agents working in one
// directory is ordinary, and neither is evidence about the other.
func TestNewestUnclaimedSessionSkipsAClaimedRival(t *testing.T) {
	dir := sessionDir(t)
	now := time.Now()
	boundLast := now.Add(-5 * time.Hour)
	bound := writeSession(t, dir, "bound", boundLast, boundLast)
	writeSession(t, dir, "rival", now.Add(-3*time.Minute), now.Add(-3*time.Minute))

	rec := resume.Record{Agent: "claude", SessionID: "bound"}
	claimed := map[string]bool{"bound": true, "rival": true}
	if got := newestUnclaimedSession(bound, rec, claimed, boundLast.Unix()); got != 0 {
		t.Fatalf("a rival owned by another pane was reported: got %d, want 0", got)
	}
}

// Old files in the same folder are not a continuation. Pane %17 on the real fleet is
// bound to a 22-day-old session sharing its directory with an inert one from two
// weeks earlier; reporting that as a stale binding is noise.
func TestNewestUnclaimedSessionIgnoresOlderLeftovers(t *testing.T) {
	dir := sessionDir(t)
	now := time.Now()
	boundLast := now.Add(-2 * time.Hour)
	bound := writeSession(t, dir, "bound", boundLast, boundLast)
	old := now.Add(-30 * 24 * time.Hour)
	writeSession(t, dir, "leftover", old, old)

	rec := resume.Record{Agent: "claude", SessionID: "bound"}
	if got := newestUnclaimedSession(bound, rec, map[string]bool{"bound": true}, boundLast.Unix()); got != 0 {
		t.Fatalf("an older leftover was reported as newer: got %d, want 0", got)
	}
}
