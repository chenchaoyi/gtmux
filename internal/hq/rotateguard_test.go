package hq

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func rotateForTest(t *testing.T, p *spyPane) (string, bool, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	r, ok, held := queueHQRotation(rotationRequest{Pane: "%6", Agent: "codex", Retiring: "old-session", Input: "/new", CreatedAt: now.Unix()})
	if !ok {
		return "", false, held
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), func(string) (string, time.Time) {
		return "task_complete", now.Add(-time.Minute)
	}, now)
	if len(p.pasted) == 0 {
		return "", false, "unsafe input or active turn"
	}
	return p.pasted[0], true, ""
}

// box renders a capture the draft reader can parse, holding draft.
func box(draft string) string {
	return "some transcript\n╭──────────────────╮\n│ ❯ " + draft + "   │\n╰──────────────────╯"
}

type spyPane struct {
	screen   string
	inMode   bool
	pasted   []string
	enters   int
	enterErr error
	onPaste  func()
}

func (s *spyPane) io() dispatch.IO {
	return dispatch.IO{
		CaptureColor: func() string { return s.screen },
		InMode:       func() bool { return s.inMode },
		Paste: func(t string) error {
			if s.onPaste != nil {
				s.onPaste()
			}
			s.pasted = append(s.pasted, t)
			return nil
		},
		Enter:    func() error { s.enters++; return s.enterErr },
		ExitMode: func() error { return nil },
		Sleep:    func() {},
	}
}

// The incident: a rotation typed `/clear` into a box holding a half-written `%11 `, and
// the Enter submitted `%11 /clear` — which HQ read as an instruction and carried out on
// a live session. A rotation must never be the thing that submits someone's sentence.
func TestRotationWillNotTypeOverADraft(t *testing.T) {
	for _, draft := range []string{"%11 ", "%11 /clear", "half a thought"} {
		p := &spyPane{screen: box(draft)}
		input, ok, held := rotateForTest(t, p)
		if ok || input != "" {
			t.Errorf("draft %q: rotated anyway (input=%q)", draft, input)
		}
		if len(p.pasted) != 0 || p.enters != 0 {
			t.Errorf("draft %q: typed into the box — pasted=%v enters=%d", draft, p.pasted, p.enters)
		}
		if held == "" {
			t.Errorf("draft %q: withheld silently — the caller has nothing to tell the user", draft)
		}
	}
}

// Copy/view-mode swallows keys as navigation, and a capture with no locatable input box
// is a screen we cannot read. Both are answered "not empty" — never typing is the safe
// side of every question this guard cannot settle.
func TestRotationHoldsWhenTheBoxCannotBeRead(t *testing.T) {
	cases := map[string]*spyPane{
		"copy-mode":    {screen: box(""), inMode: true},
		"no input box": {screen: "just scrollback, no box at all\nnothing here"},
	}
	for name, p := range cases {
		if _, ok, _ := rotateForTest(t, p); ok {
			t.Errorf("%s: rotated on a screen it could not read", name)
		}
		if len(p.pasted) != 0 || p.enters != 0 {
			t.Errorf("%s: typed anyway", name)
		}
	}
}

// And it still rotates when the box is genuinely empty — a guard that never lets the
// act through is not a fix, it is a different outage.
func TestRotationProceedsOnAnEmptyBox(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // rotateHQ resolves the HQ home to journal the act
	p := &spyPane{screen: box("")}
	_, ok, held := rotateForTest(t, p)
	if !ok {
		t.Fatalf("held on an empty box: %q", held)
	}
	if len(p.pasted) != 1 || !strings.HasPrefix(p.pasted[0], "/") {
		t.Errorf("pasted %v, want the agent's own reset command", p.pasted)
	}
	if p.enters != 1 {
		t.Errorf("enters=%d, want exactly one submit", p.enters)
	}
}

func testRotationRequest(t *testing.T, now time.Time) rotationRequest {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	r, ok, reason := queueHQRotation(rotationRequest{Pane: "%6", Agent: "codex",
		Retiring: "old-session", Input: "/new", CreatedAt: now.Unix()})
	if !ok {
		t.Fatal(reason)
	}
	return r
}

func testBoundary(kind string, at time.Time) func(string) (string, time.Time) {
	return func(string) (string, time.Time) { return kind, at }
}

func testRotationEvents(t *testing.T) []events.Record {
	t.Helper()
	recs, gap := events.ReadSince(0)
	if gap {
		t.Fatal("unexpected journal gap")
	}
	return recs
}

func TestRotationWaitsForTaskEndAndConfirmsSuccessor(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	started := testBoundary("task_started", now)
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), started, now)
	if len(p.pasted) != 0 {
		t.Fatal("active Codex task accepted /new")
	}
	if err := state.WriteMarker(state.ActivePath("%6"), "old-session"); err != nil {
		t.Fatal(err)
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	if len(p.pasted) != 0 {
		t.Fatal("active hook marker was ignored")
	}
	state.Remove(state.ActivePath("%6"))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	if len(p.pasted) != 1 || p.pasted[0] != "/new" || p.enters != 1 {
		t.Fatalf("delivery = %v / %d", p.pasted, p.enters)
	}
	sent, exists, err := readRotationRequest()
	if err != nil || !exists || sent.SentAt == 0 {
		t.Fatalf("sent state = %+v %v %v", sent, exists, err)
	}
	advanceHQRotation(sent, "%6", "codex", "old-session", p.io(), complete, now.Add(time.Second))
	if len(testRotationEvents(t)) != 1 { // request only; a pasted key is not success
		t.Fatal("rotation claimed success before the new session ID")
	}
	advanceHQRotation(sent, "%6", "codex", "new-session", p.io(), complete, now.Add(2*time.Second))
	if _, exists, _ := readRotationRequest(); exists || len(p.pasted) != 1 {
		t.Fatal("settled request was not cleared or reset was repeated")
	}
	recs := testRotationEvents(t)
	if len(recs) != 2 || recs[1].Event != events.AuditEventRotate ||
		recs[1].PreviousAgentSession != "old-session" || recs[1].AgentSession != "new-session" {
		t.Fatalf("confirmed receipt = %+v", recs)
	}
}

func TestRotationDuplicateRequestDoesNotDuplicateKeysOrAudit(t *testing.T) {
	r := testRotationRequest(t, time.Now())
	again, ok, reason := queueHQRotation(r)
	if !ok || reason != "" || again != r || len(testRotationEvents(t)) != 1 {
		t.Fatalf("duplicate = %+v, ok=%v reason=%q", again, ok, reason)
	}
	other := r
	other.Retiring = "another-session"
	if _, ok, reason := queueHQRotation(other); ok || !strings.Contains(reason, "still pending") {
		t.Fatalf("second session replaced an unresolved request: ok=%v reason=%q", ok, reason)
	}
}

func TestRotationFailureReceiptAndNoBlindRetry(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box(""), enterErr: errors.New("send failed")}
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	if _, exists, _ := readRotationRequest(); exists {
		t.Fatal("failed Enter left an automatic retry armed")
	}
	recs := testRotationEvents(t)
	if len(recs) != 2 || recs[1].Event != events.AuditEventRotateFailed || !strings.Contains(recs[1].Summary, "inspect HQ input box") {
		t.Fatalf("failure receipt = %+v", recs)
	}
}

func TestRotationPersistsAttemptBeforeTyping(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	p.onPaste = func() {
		sent, exists, err := readRotationRequest()
		if err != nil || !exists || sent.SentAt == 0 {
			t.Errorf("reset input was pasted before attempt became durable: %+v %v %v", sent, exists, err)
		}
	}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(),
		testBoundary("task_complete", now.Add(-time.Minute)), now)
	if len(p.pasted) != 1 || p.enters != 1 {
		t.Fatalf("reset was not submitted: %+v", p)
	}
}

func TestRotationRejectedResetTimesOutWithoutSuccess(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now)
	sent, _, _ := readRotationRequest()
	advanceHQRotation(sent, "%6", "codex", "old-session", p.io(), complete, now.Add(rotationSettleWait))
	recs := testRotationEvents(t)
	if len(recs) != 2 || recs[1].Event != events.AuditEventRotateFailed || len(p.pasted) != 1 {
		t.Fatalf("timeout = %+v, keys=%v", recs, p.pasted)
	}
}

func TestRotationDraftExpiresWithoutSubmittingOrLosingFailureReceipt(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("unsent user draft")}
	complete := testBoundary("task_complete", now.Add(-time.Minute))
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now.Add(rotationQueueLimit-time.Second))
	if len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("rotation touched a user draft")
	}
	// A later tick cannot leave the request stuck forever behind the draft.
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), complete, now.Add(rotationQueueLimit))
	if _, exists, _ := readRotationRequest(); exists {
		t.Fatal("expired request stayed pending")
	}
	recs := testRotationEvents(t)
	if len(recs) != 2 || recs[1].Event != events.AuditEventRotateFailed || !strings.Contains(recs[1].Summary, "no safe idle input box") {
		t.Fatalf("missing failure receipt: %+v", recs)
	}
}

func TestRotationChangedSessionBeforeDeliveryDoesNotResetSuccessor(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	advanceHQRotation(r, "%6", "codex", "unexpected-successor", p.io(),
		testBoundary("task_complete", now.Add(-time.Minute)), now)
	if len(p.pasted) != 0 || p.enters != 0 {
		t.Fatal("stale request reset an unrelated successor")
	}
	recs := testRotationEvents(t)
	if len(recs) != 2 || recs[1].Event != events.AuditEventRotateFailed {
		t.Fatalf("stale request lacked failure receipt: %+v", recs)
	}
}

func stubRotationEvents(t *testing.T, recs ...events.Record) {
	t.Helper()
	prev := rotationEvents
	rotationEvents = func(int64, int64) []events.Record { return recs }
	t.Cleanup(func() { rotationEvents = prev })
}

// A fresh Codex session's id reaches the resume record only after its first finished
// turn, but its SessionStart and first prompt reach the event stream at once. A sent reset
// is confirmed from those while the resume record still names the retiring session
// (2026-10-04: a working rotation audited as failed at 45 s; the resume record changed at
// 81 s).
func TestRotationConfirmsFromTheEventStream(t *testing.T) {
	for _, ev := range []string{"SessionStart", "UserPromptSubmit"} {
		t.Run(ev, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			r.SentAt = now.Unix()
			if err := writeRotationFile(r, false); err != nil {
				t.Fatal(err)
			}
			stubRotationEvents(t, events.Record{Ts: now.Unix() + 1, Event: ev, Pane: "%6", Agent: "Codex", AgentSession: "new-session"})
			p := &spyPane{screen: box("")}
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_complete", now), now.Add(2*time.Second))
			if _, exists, _ := readRotationRequest(); exists {
				t.Fatal("the rotation is still pending")
			}
			recs := testRotationEvents(t)
			last := recs[len(recs)-1]
			if last.Event != events.AuditEventRotate || last.AgentSession != "new-session" || last.PreviousAgentSession != "old-session" {
				t.Fatalf("confirmed receipt = %+v", last)
			}
			if len(p.pasted) != 0 {
				t.Fatal("confirming must not type anything")
			}
		})
	}
}

// Only the new session's own events, on the HQ pane, after the reset was sent, count.
func TestRotationEventsThatDoNotConfirm(t *testing.T) {
	sent := int64(1_791_106_000)
	r := rotationRequest{Pane: "%6", Agent: "codex", Retiring: "old-session", Input: "/new", SentAt: sent}
	good := events.Record{Ts: sent + 1, Event: "SessionStart", Pane: "%6", Agent: "Codex", AgentSession: "new-session"}
	if got := rotationSessionFromEvents(r, []events.Record{good}); got != "new-session" {
		t.Fatalf("the control does not confirm: %q", got)
	}
	for name, e := range map[string]events.Record{
		"before the reset was sent": {Ts: sent - 1, Event: "SessionStart", Pane: "%6", Agent: "Codex", AgentSession: "new-session"},
		"another pane":              {Ts: sent + 1, Event: "SessionStart", Pane: "%7", Agent: "Codex", AgentSession: "new-session"},
		"a native session":          {Ts: sent + 1, Event: "SessionStart", Pane: "", Agent: "Codex", AgentSession: "new-session"},
		"another agent":             {Ts: sent + 1, Event: "SessionStart", Pane: "%6", Agent: "Claude Code", AgentSession: "new-session"},
		"the retiring session":      {Ts: sent + 1, Event: "UserPromptSubmit", Pane: "%6", Agent: "Codex", AgentSession: "old-session"},
		"no session id":             {Ts: sent + 1, Event: "UserPromptSubmit", Pane: "%6", Agent: "Codex"},
		"a Stop, not a start":       {Ts: sent + 1, Event: "Stop", Pane: "%6", Agent: "Codex", AgentSession: "new-session"},
	} {
		if got := rotationSessionFromEvents(r, []events.Record{e}); got != "" {
			t.Errorf("%s confirmed %q", name, got)
		}
	}
	notSent := r
	notSent.SentAt = 0
	if got := rotationSessionFromEvents(notSent, []events.Record{good}); got != "" {
		t.Fatalf("a reset never sent was confirmed: %q", got)
	}
}

// Before the reset is sent, a new session's events change nothing: the request is not
// confirmed, and the existing guards decide as before.
func TestRotationNotSentIsNotConfirmedByEvents(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	stubRotationEvents(t, events.Record{Ts: now.Unix() + 1, Event: "SessionStart", Pane: "%6", Agent: "Codex", AgentSession: "new-session"})
	p := &spyPane{screen: box("")}
	advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_started", now), now.Add(time.Second))
	if _, exists, _ := readRotationRequest(); !exists {
		t.Fatal("an unsent rotation was settled")
	}
	for _, rec := range testRotationEvents(t) {
		if rec.Event == events.AuditEventRotate && rec.AgentSession == "new-session" {
			t.Fatal("an unsent rotation was confirmed")
		}
	}
}
