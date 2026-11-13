package codexbridge

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCBA004PublicationAndAttachmentFinishBeforeNewEpochActivation(t *testing.T) {
	var epoch connectionEpoch
	old := epoch.reserve()
	if !epoch.activate(old) {
		t.Fatal("initial epoch")
	}
	b := newConnectionBinding(context.Background(), resume.CodexFrontend{Sequence: old}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	defer b.close()
	now := time.Now()
	b.now = func() time.Time { return now }
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var late atomic.Bool
	calls := 0
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		calls++
		if calls == 1 {
			return f, state.ErrLeaseHeld
		}
		close(entered)
		<-release
		late.Store(!epoch.current(old))
		f.SessionID = id
		return f, nil
	}
	b.stage("old-root", "")
	now = now.Add(time.Second)
	go func() { b.tick(); close(done) }()
	<-entered
	newer := epoch.reserve()
	// A bounded admission wait is the contract, not a sleep/scheduling guess.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	active, err := completeAdmission(ctx, &epoch, newer)
	cancel()
	if active || !errors.Is(err, context.DeadlineExceeded) || !epoch.current(old) {
		close(release)
		<-done
		t.Fatal("activation passed an in-flight publisher", active, err)
	}
	close(release)
	<-done
	if late.Load() || b.attachment.SessionID != "old-root" {
		t.Fatal("publication did not linearize before activation")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if active, err = completeAdmission(ctx, &epoch, newer); !active || err != nil {
		t.Fatal(active, err)
	}
	b.tick()
	if calls != 2 || epoch.current(old) {
		t.Fatal("retired publisher retried", calls)
	}
}

func TestCBA004CanceledPublicationWaitNeverStartsBind(t *testing.T) {
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	ctx, cancel := context.WithCancel(context.Background())
	b := newConnectionBinding(ctx, resume.CodexFrontend{Sequence: id}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	defer b.close()
	calls := 0
	b.bind = func(f resume.CodexFrontend, _, _ string) (resume.CodexFrontend, error) { calls++; return f, nil }
	cancel()
	b.stage("root", "")
	b.tick()
	if calls != 0 {
		t.Fatal("canceled connection entered Bind", calls)
	}
}

func TestCBA004CancellationInsidePublicationPermitDoesNotRetry(t *testing.T) {
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	ctx, cancel := context.WithCancel(context.Background())
	b := newConnectionBinding(ctx, resume.CodexFrontend{Sequence: id}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	defer b.close()
	calls := 0
	b.bind = func(f resume.CodexFrontend, _, _ string) (resume.CodexFrontend, error) { calls++; return f, nil }
	publish := b.publish
	b.publish = func(commit func()) error {
		return publish(func() { cancel(); commit() })
	}
	b.stage("root", "")
	b.tick()
	if calls != 0 || b.pending != nil || b.attachment.SessionID != "" {
		t.Fatal("cancellation at publication retained a retry or authority", calls, b.pending, b.attachment)
	}
}

func TestCBA004HeartbeatPublicationAlsoPrecedesNewEpoch(t *testing.T) {
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	b := newConnectionBinding(context.Background(), resume.CodexFrontend{Sequence: id, SessionID: "root"}, &epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	defer b.close()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { close(entered); <-release; return f, nil }
	go func() { b.tick(); close(done) }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	active, err := completeAdmission(ctx, &epoch, epoch.reserve())
	cancel()
	close(release)
	<-done
	if active || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("heartbeat committed after new activation", active, err)
	}
}
