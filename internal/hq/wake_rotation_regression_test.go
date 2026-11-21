package hq

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
)

func TestHWA008ChangedSuccessorFailsIndependentlyWithoutSecondReset(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := testRotationRequest(t, now)
	prior := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = prior })
	var independent []notify.Options
	wakeDesktopNotice = func(opt notify.Options) bool {
		independent = append(independent, opt)
		return false
	}
	p := &spyPane{screen: box("")}
	advanceHQRotation(r, "%6", "codex", r.Retiring, p.io(), testBoundary("task_complete", now.Add(-time.Minute)), now.Add(time.Second))
	started := testBoundary("task_started", now.Add(2*time.Second))
	advanceHQRotation(r, "%6", "codex", "transient-successor", p.io(), started, now.Add(3*time.Second))
	if saved, ok, err := readRotationRequest(); err != nil || !ok || saved.Successor != "transient-successor" {
		t.Fatalf("missing or prematurely confirmed successor: %+v %v %v", saved, ok, err)
	}
	writeRotationHistory(t, "different-successor")
	advanceHQRotation(r, "%6", "codex", "different-successor", p.io(), started, now.Add(4*time.Second))
	advanceHQRotation(r, "%6", "codex", r.Retiring, p.io(), started, now.Add(5*time.Second))
	if _, ok, err := readRotationRequest(); err != nil || ok || len(p.pasted) != 1 || p.enters != 1 {
		t.Fatal("stale rotation repeated input or retained its reservation", err, ok, p.pasted, p.enters)
	}
	records := testRotationEvents(t)
	if len(independent) != 1 || !strings.Contains(independent[0].Message, "changed again") ||
		len(records) != 2 || records[1].Event != events.AuditEventRotateFailed ||
		!strings.Contains(records[1].Summary, "changed again") {
		t.Fatalf("failure was not independently journaled once: %#v", records)
	}
}
