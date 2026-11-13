package codexbridge

import (
	"context"
	"errors"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestCBA006ProductionFactoryHeartbeatRetainsConnectionContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	b := newConnectionBinding(ctx, resume.CodexFrontend{Sequence: id}, &epoch, nil)
	defer b.close()
	cancel()
	// Call the actual stored production heartbeat callback. A disconnected
	// callback must report cancellation before even validating a fresh identity.
	if _, err := b.refresh(b.attachment); !errors.Is(err, context.Canceled) {
		t.Fatal("production heartbeat lost its connection context", err)
	}
}
