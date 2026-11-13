package codexbridge

import (
	"context"
	"io"
	"time"
)

// This is Run's actual accepted-connection lifecycle. Retire publication before
// transport close can release already-read observer frames during drain.
func runFrontendLifetime(ctx context.Context, ticks <-chan time.Time, down, up io.Closer, ended <-chan error, binding *frontendBinding) {
	stop := func(remaining int) {
		binding.close()
		_ = down.Close()
		_ = up.Close()
		for range remaining {
			<-ended
		}
	}
	for {
		if ctx.Err() != nil {
			stop(2)
			return
		}
		select {
		case <-ended:
			stop(1)
			return
		case <-ctx.Done():
			stop(2)
			return
		case <-ticks:
			if ctx.Err() == nil {
				binding.tick()
			}
		}
	}
}
