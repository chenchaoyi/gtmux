package launchhost

import (
	"context"
	"testing"
	"time"
)

func TestCFR006AgentReadReturnsBeforeBlockedReaderSettles(t *testing.T) {
	old := agentProcessRead
	t.Cleanup(func() { agentProcessRead = old })
	entered, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	agentProcessRead = func(context.Context, int) (process, bool) {
		close(entered)
		<-release
		close(settled)
		return process{PID: 77, Command: "/opt/codex", Start: "birth"}, true
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type answer struct {
		p  AgentProcess
		ok bool
	}
	out := make(chan answer, 1)
	go func() { p, ok := ReadAgentContext(ctx, 77); out <- answer{p, ok} }()
	<-entered
	cancel()
	select {
	case got := <-out:
		if got.ok || got.p.PID != 0 {
			t.Fatal("late authority", got)
		}
	case <-time.After(time.Second):
		close(release)
		<-settled
		t.Fatal("reader blocked past canceled parent")
	}
	close(release)
	<-settled
	agentProcessRead = func(context.Context, int) (process, bool) {
		return process{PID: 77, Command: "/opt/codex", Start: "birth"}, true
	}
	p, ok := ReadAgentContext(context.Background(), 77)
	if !ok || p.PID != 77 || p.Comm != "codex" || p.Start != "birth" {
		t.Fatal(p, ok)
	}
}
