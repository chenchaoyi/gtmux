package hq

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
)

func TestRotationRetainsRequestBehindWorkspaceAndCannotReplayOldSnapshot(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	defer func() { release() }()
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	saved, exists, err := readRotationRequest()
	if err != nil || !exists || saved.SentAt != 0 || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("contended rotation altered input or lost its request", saved, p)
	}
	release()
	release = func() {}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now.Add(time.Second))
	if len(p.pasted) != 1 || p.enters != 1 {
		t.Fatal("old unsent snapshot repeated reset", p)
	}
	advanceHQRotation(r, "%6", "codex", "new-session", p.io(), complete, now.Add(2*time.Second))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now.Add(3*time.Second))
	if _, exists, _ := readRotationRequest(); exists || len(p.pasted) != 1 {
		t.Fatal("settled request was replayed", p)
	}
}

func TestStartupBriefingRefusesCompetingInputWithoutClaimingDelivery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	release, acquired, err := hqinput.Acquire()
	if err != nil || !acquired {
		t.Fatal(acquired, err)
	}
	defer release()
	p := &spyPane{screen: box("")}
	io := p.io()
	io.Capture = func() string { return p.screen }
	io.Now = func() int64 { return 100 }
	io.Events = func(int64) []dispatch.Ev {
		return []dispatch.Ev{{Kind: dispatch.EvSubmit, Head: dispatch.NormalizeHead("startup"), Ts: 100}}
	}
	got := deliverHQBriefingInput(io, dispatch.Opts{HookEquipped: true, DeliverTimeout: 1}, "startup")
	if got.Delivered || got.State != dispatch.StateRefusedWaiting || got.Evidence != "hq-input-locked" || len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("startup raced input owner or claimed delivery", got, p)
	}
}
