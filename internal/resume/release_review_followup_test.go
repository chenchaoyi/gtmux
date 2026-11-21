package resume

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCRF001MovedFrontendRefreshesRetainedNativeReceipt(t *testing.T) {
	f, pane, _ := locationFixture(t)
	recordCrossPaneBinding(t, f)
	if _, exists := native.Load(f.SessionID, f.HomeID); exists {
		t.Fatal("fixture must have a binding but no native observation")
	}
	pane.Loc = "moved:0.1"
	moved, err := RefreshCodexFrontend(f)
	if err != nil {
		t.Fatal(err)
	}
	got, exists := native.BindingFor(f.SessionID, f.HomeID)
	if !exists || got.Loc != moved.Loc || got.Pane != f.Pane || got.ID != f.Instance {
		t.Fatalf("receipt did not follow the verified owner: %+v", got)
	}
	if _, exists := native.Load(f.SessionID, f.HomeID); exists {
		t.Fatal("moving the binding fabricated a native observation")
	}
}

func TestCRF002ExactClosedReceiptReleasesRelocationDestination(t *testing.T) {
	f, pane, _ := locationFixture(t)
	pane.Loc = "closed:0.0"
	prior := f
	prior.Loc, prior.Pane, prior.PID, prior.PanePID = pane.Loc, "%19", 201, 200
	prior.Instance, prior.SessionID, prior.Retired = strings.Repeat("c", 32), "closed-thread", true
	if err := writeBindingJSON(frontendPath(prior.Instance), prior); err != nil {
		t.Fatal(err)
	}
	if err := writeBindingJSON(fileFor(prior.Loc), frontendRecord(prior)); err != nil {
		t.Fatal(err)
	}
	if _, live := LoadLive(prior.Loc); live {
		t.Fatal("closed receipt conferred live authority")
	}
	moved, err := RefreshCodexFrontend(f)
	if err != nil {
		t.Fatal(err)
	}
	if got, live := LoadLive(pane.Loc); !live || got.SessionID != f.SessionID || moved.Instance != f.Instance {
		t.Fatalf("verified move did not reuse the closed destination: %+v", got)
	}
}

func TestCRF003ObservedSaveRetriesTemporaryPublicationContention(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	record := Record{Agent: "claude", SessionID: "observed-thread", UpdatedAt: time.Now().Unix()}
	done := make(chan error, 1)
	go func() { done <- SaveObserved("worker:0.0", record) }()
	select {
	case err := <-done:
		t.Fatalf("observed save gave up during brief contention: %v", err)
	case <-time.After(120 * time.Millisecond):
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observed save did not resume after release")
	}
	if saved, ok := Load("worker:0.0"); !ok || saved != record {
		t.Fatalf("observed record was lost: %+v", saved)
	}
}
