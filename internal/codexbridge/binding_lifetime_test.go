package codexbridge

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type bindingLifetimeCloser func() error

func (f bindingLifetimeCloser) Close() error { return f() }

func TestCBA005RunDisconnectRetiresBeforeTransportDrain(t *testing.T) {
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	b := newConnectionBinding(context.Background(), resume.CodexFrontend{Sequence: id}, &epoch, nil)
	var retired atomic.Bool
	b.end = func(resume.CodexFrontend) { retired.Store(true) }
	o, _ := readyObserver(t)
	o.bind = b.stage
	o.selectRoot = b.selectRoot
	o.resetHome = b.resetHome
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	var calls atomic.Int32
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		calls.Add(1)
		f.SessionID = id
		return f, nil
	}
	closed := make(chan struct{})
	ended := make(chan error, 2)
	ended <- nil
	go func() { <-closed; o.response(rootResponse(1, "late-root")); ended <- nil }()
	down := bindingLifetimeCloser(func() error {
		if !retired.Load() {
			t.Error("transport closed before retirement")
		}
		return nil
	})
	up := bindingLifetimeCloser(func() error { close(closed); return nil })
	runFrontendLifetime(context.Background(), nil, down, up, ended, b)
	b.close()
	b.tick()
	o.response(rootResponse(1, "late-root"))
	if calls.Load() != 0 {
		t.Fatal("draining response published", calls.Load())
	}
}

func TestCBA005AlreadyCanceledRunAndReadyTickCannotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	b := newConnectionBinding(ctx, resume.CodexFrontend{Sequence: id}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	now := time.Now()
	b.now = func() time.Time { return now }
	calls := 0
	b.bind = func(f resume.CodexFrontend, _, _ string) (resume.CodexFrontend, error) {
		calls++
		return f, state.ErrLeaseHeld
	}
	b.stage("root", "")
	now = now.Add(time.Second)
	ticks := make(chan time.Time, 1)
	ticks <- now
	ended := make(chan error, 2)
	down := bindingLifetimeCloser(func() error { ended <- nil; ended <- nil; return nil })
	up := bindingLifetimeCloser(func() error { return nil })
	cancel()
	runFrontendLifetime(ctx, ticks, down, up, ended, b)
	b.tick()
	b.stage("late-root", "")
	if calls != 1 {
		t.Fatal("canceled Run entered another Bind", calls)
	}
}

func TestCBA005StopWaitsForPublicationBeforeClosingAndRemainsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	b := newConnectionBinding(ctx, resume.CodexFrontend{Sequence: id}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	b.bind = func(f resume.CodexFrontend, _, _ string) (resume.CodexFrontend, error) {
		close(entered)
		<-release
		return f, b.ctx.Err()
	}
	go func() { b.stage("root", ""); close(done) }()
	<-entered
	cancel()
	close(release)
	<-done
	ended := make(chan error, 2)
	var closes atomic.Int32
	closer := bindingLifetimeCloser(func() error {
		if !b.closed {
			t.Error("close before publication fence")
		}
		closes.Add(1)
		ended <- nil
		return nil
	})
	runFrontendLifetime(ctx, nil, closer, closer, ended, b)
	if closes.Load() != 2 || b.attachment.SessionID != "" || !b.closed {
		t.Fatal("stopped connection retained publication", closes.Load(), b.attachment.SessionID)
	}
}
