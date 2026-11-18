package app

import (
	"context"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

const restoreSerialBudget = 1500 * time.Millisecond
const restoreSerialPoll = 50 * time.Millisecond
const restoreSerialMaxObservations = 32

func restoreSerialLess(loc, pane, otherLoc, otherPane string) bool {
	if loc != otherLoc {
		return loc < otherLoc
	}
	return pane < otherPane
}

// No lease or mutex spans observation, publication, or a cancellable timer.
// Every eligible pane shares the former fleet-wide 1.5s grace budget.
type restoreSerialGate struct {
	ctx       context.Context
	mode      resumeMode
	slots     int
	remaining time.Duration
	stopped   bool
	now       func() time.Time
	pause     func(context.Context, time.Duration) bool
	observe   func(resumeAttempt) bool
	publish   func(resumeAttempt)
	stop      func() bool
}

func newRestoreSerialGate(ctx context.Context, mode resumeMode, slots int) *restoreSerialGate {
	return &restoreSerialGate{ctx: ctx, mode: mode, slots: slots, remaining: restoreSerialBudget,
		now: time.Now, pause: pauseRestoreSerial,
		observe: func(a resumeAttempt) bool {
			current, live := resume.LoadLive(a.Loc)
			return resumeAttemptConfirmed(a, current, live)
		},
		publish: func(resumeAttempt) {}, stop: func() bool { return false },
	}
}

func pauseRestoreSerial(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (g *restoreSerialGate) cancelled() bool {
	if g.stopped || g.ctx.Err() != nil || g.stop() {
		g.stopped = true
	}
	return g.stopped
}

// run is also the production ordering seam: start cannot run for B until A's
// publication and ready/timeout/cancel decision have returned.
func (g *restoreSerialGate) run(start func() (resumeAttempt, bool)) (resumeAttempt, bool) {
	if g.cancelled() {
		return resumeAttempt{}, false
	}
	slots := g.slots
	if slots < 1 {
		slots = 1
	}
	if g.slots > 0 {
		g.slots--
	}
	a, sent := start()
	if a.Pane == "" {
		return a, sent
	}
	g.publish(a)
	if g.mode != resumeAuto || (a.State != "pending" && a.State != "unknown" && a.State != "send-failed") {
		return a, sent
	}
	began := g.now()
	allowance := g.remaining / time.Duration(slots)
	deadline := began.Add(allowance)
	defer func() {
		spent := g.now().Sub(began)
		if spent < 0 {
			spent = 0
		}
		g.remaining -= spent
		if g.remaining < 0 {
			g.remaining = 0
		}
	}()
	confirmable := a.State == "pending"
	a.IssueState = a.State
	a.State = "startup-timeout"
	for observations := 0; observations < restoreSerialMaxObservations; observations++ {
		if g.cancelled() {
			a.State = "cancelled"
			break
		}
		// The attempt must retain pending for the native confirmation CAS.
		pending := a
		pending.State = "pending"
		if confirmable && g.observe(pending) {
			a.State = "confirmed"
			break
		}
		// Observe once after the last pause, including at the deadline.
		left := deadline.Sub(g.now())
		if left <= 0 {
			break
		}
		if left > restoreSerialPoll {
			left = restoreSerialPoll
		}
		if !g.pause(g.ctx, left) {
			g.stopped = true
			a.State = "cancelled"
			break
		}
	}
	g.publish(a)
	return a, sent
}
