package app

import (
	"context"
	"github.com/chenchaoyi/gtmux/internal/screenshot"
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestASC007TargetReadStopsAtCancellationBeforeMintOrSend(t *testing.T) {
	for _, op := range []string{"target", "send"} {
		t.Run(op, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			target := screenshot.Target{Version: 1, Pane: "%9", Loc: "synthetic:0.0", Agent: "codex", SessionID: "s", HomeID: "h", Instance: "i", ClientPID: 100, PanePID: 99, ServerPID: 90, ServerStart: "birth"}
			token := ""
			if op == "send" {
				var e error
				token, e = screenshot.MintTarget(target, time.Now())
				if e != nil {
					t.Fatal(e)
				}
			}
			old := screenshotCurrentTarget
			t.Cleanup(func() { screenshotCurrentTarget = old })
			entered, release, settled := make(chan struct{}), make(chan struct{}), make(chan struct{})
			screenshotCurrentTarget = func(string) (screenshot.Target, error) { close(entered); <-release; close(settled); return target, nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if op == "target" {
					_, e := screenshotResolveTarget(ctx, "%9", "")
					done <- e
				} else {
					_, e := screenshotSend(ctx, "not-read.png", token, "synthetic-deadline-001", "")
					done <- e
				}
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
				t.Fatal("target blocked past cancellation")
			}
			close(release)
			<-settled
			if op == "target" {
				if _, e := os.Stat(filepath.Join(state.Dir(), "capture-requests")); !os.IsNotExist(e) {
					t.Fatal("canceled read minted metadata")
				}
			} else {
				if _, e := screenshot.ReadReceipt("synthetic-deadline-001"); e == nil {
					t.Fatal("canceled read claimed submission")
				}
			}
		})
	}
}
