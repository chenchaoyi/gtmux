package screenshot

import (
	"context"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"testing"
	"time"
)

func TestASC007HostObservationCannotAuthorizeAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		v, e := ReadWithin(ctx, func() bool { close(entered); <-release; close(settled); return true })
		if v {
			done <- Fail("fixture", "late authorization")
			return
		}
		done <- e
	}()
	<-entered
	cancel()
	select {
	case e := <-done:
		if e != context.Canceled {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		close(release)
		<-settled
		t.Fatal("blocked past caller cancellation")
	}
	close(release)
	<-settled
	v, e := ReadWithin(context.Background(), func() int { return 7 })
	if e != nil || v != 7 {
		t.Fatal(v, e)
	}
}
func TestASC007HostIncarnationUsesExactUTCMetadata(t *testing.T) {
	h := Host{PID: 77, Start: "2026-11-10T01:02:03.250Z"}
	p := launchhost.AgentProcess{PID: 77, Comm: "GtmuxBar", Start: "Tue Nov 10 01:02:03 2026"}
	if !hostProcessMatches(h, p) {
		t.Fatal("UTC birth did not match")
	}
	for _, change := range []func(*launchhost.AgentProcess){func(v *launchhost.AgentProcess) { v.PID++ }, func(v *launchhost.AgentProcess) { v.Comm = "other" }, func(v *launchhost.AgentProcess) { v.Start = "Tue Nov 10 01:02:02 2026" }, func(v *launchhost.AgentProcess) { v.Start = "Tue Nov 10 09:02:03 2026" }} {
		v := p
		change(&v)
		if hostProcessMatches(h, v) {
			t.Fatal("different host accepted", v)
		}
	}
}
