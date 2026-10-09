package sessionpolicy

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func fixture(t *testing.T, id, origin string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	text := `{"type":"session_meta","payload":{"id":"` + id + `","originator":"` + origin + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-09T00-00-00-"+id+".jsonl"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func setup(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	fixture(t, "desk", "codex_work_desktop")
}

func TestConsentIsPerConversationAndReversible(t *testing.T) {
	setup(t)
	fixture(t, "other", "Codex Desktop")
	fixture(t, "tui", "codex-tui")
	if Get("desk").HQ || ObserveEvent("", "Codex", "desk", "", 200) {
		t.Fatal("detection must not imply follow")
	}
	v, err := saveAt("desk", Settings{HQ: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if v.Notify || v.Knowledge || v.FollowSince != 101 {
		t.Fatalf("default opt-in = %+v", v)
	}
	if ObserveEvent("", "Codex", "desk", "", 99) || !ObserveEvent("", "Codex", "desk", "", 101) || Get("other").HQ {
		t.Fatal("observation crossed identity/consent boundary")
	}
	v.Notify, v.Knowledge = true, true
	v, err = saveAt("desk", v, 120)
	if err != nil {
		t.Fatal(err)
	}
	if v.FollowSince != 101 || v.KnowledgeSince != 121 || LearnEvent("", "Codex", "desk", "", 120) || !LearnEvent("", "Codex", "desk", "", 122) {
		t.Fatalf("learning window = %+v", v)
	}
	v.HQ = false
	v, err = saveAt("desk", v, 130)
	if err != nil {
		t.Fatal(err)
	}
	if v.HQ || v.Notify || v.Knowledge || v.FollowSince != 0 || v.KnowledgeSince != 0 {
		t.Fatalf("stopped = %+v", v)
	}
	v.HQ = true
	v, err = saveAt("desk", v, 150)
	if err != nil {
		t.Fatal(err)
	}
	if v.FollowSince != 151 || ObserveEvent("", "Codex", "desk", "", 140) {
		t.Fatal("re-enrollment replayed earlier interval")
	}
	for _, id := range []string{"tui", "missing"} {
		if _, err = saveAt(id, Settings{HQ: true}, 100); !errors.Is(err, ErrUnverified) {
			t.Fatalf("%s accepted: %v", id, err)
		}
	}
	if !ObserveEvent("%7", "Codex", "desk", "chatgpt_desktop", 0) || !ObserveEvent("", "Claude Code", "claude", "", 0) || !ObserveEvent("", "Codex", "tui", "terminal", 0) {
		t.Fatal("terminal/other-agent behavior changed")
	}
}

func TestStaleConcurrentAndFailedSavesNeverPretendSuccess(t *testing.T) {
	setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := saveAt("desk", Settings{HQ: true}, 100); errs <- e }()
	}
	wg.Wait()
	close(errs)
	good, conflict := 0, 0
	for e := range errs {
		if e == nil {
			good++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if good != 1 || conflict != 1 || Get("desk").Revision != 1 {
		t.Fatalf("good=%d conflict=%d", good, conflict)
	}
	if err := os.WriteFile(path("desk"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if Get("desk").HQ {
		t.Fatal("corrupt permission must fail closed")
	}
	if _, err := saveAt("desk", Settings{HQ: true}, 101); err == nil {
		t.Fatal("corrupt existing policy was overwritten silently")
	}
}

func TestWriteFailureAndForgedPermissionTimes(t *testing.T) {
	setup(t)
	v, err := saveAt("desk", Settings{HQ: true, Knowledge: true, FollowSince: 1, KnowledgeSince: 1}, 100)
	if err != nil || v.FollowSince != 101 || v.KnowledgeSince != 101 {
		t.Fatalf("caller forged consent time: %+v %v", v, err)
	}
	fixture(t, "blocked", "codex_work_desktop")
	if err := os.Mkdir(path("blocked"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = saveAt("blocked", Settings{HQ: true}, 100); err == nil {
		t.Fatal("write/read failure reported success")
	}
	if Get("blocked").HQ {
		t.Fatal("failed save enabled follow")
	}
}

func TestSameSecondReEnrollmentHasDistinctKnowledgeInterval(t *testing.T) {
	setup(t)
	v, err := saveAt("desk", Settings{HQ: true, Knowledge: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	old := v.KnowledgeRevision
	v.HQ = false
	v, err = saveAt("desk", v, 100)
	if err != nil {
		t.Fatal(err)
	}
	v.HQ = true
	v.Knowledge = true
	v, err = saveAt("desk", v, 100)
	if err != nil {
		t.Fatal(err)
	}
	if v.KnowledgeRevision == old || v.KnowledgeSince != 101 || ObserveEvent("", "Codex", "desk", "", 100) {
		t.Fatalf("consent interval collided: %+v", v)
	}
	v.Notify = true
	v, err = saveAt("desk", v, 110)
	if err != nil {
		t.Fatal(err)
	}
	if v.KnowledgeRevision != 3 {
		t.Fatal("notification-only edit resets learning carry")
	}
}
