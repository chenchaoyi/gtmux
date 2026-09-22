package server

import (
	"sync"
	"testing"
	"time"
)

// The roster on disk must never go BACKWARDS (found in the 2026-09-22 self-check).
//
// Every mutation took its snapshot under the lock and wrote it after releasing it, so two
// requests at once could write in the opposite order from the one they changed things
// in: a stale roster lands last, and whatever the newer one did — a revoke — is gone the
// next time serve starts. One site even saved from a goroutine, which widened that window
// to "whenever the scheduler gets to it".

func TestTheRosterOnDiskNeverGoesBackwards(t *testing.T) {
	var mu sync.Mutex
	var onDisk []EnrolledDevice
	m := NewEnrollManager(nil, func(s []EnrolledDevice) { mu.Lock(); onDisk = s; mu.Unlock() })
	newer := []EnrolledDevice{{ID: "after-the-revoke"}}
	older := []EnrolledDevice{{ID: "before-the-revoke"}, {ID: "the-revoked-link"}}
	// The newer snapshot is written first; the older one arrives late.
	m.persist(2, newer)
	m.persist(1, older)
	mu.Lock()
	defer mu.Unlock()
	if len(onDisk) != 1 || onDisk[0].ID != "after-the-revoke" {
		t.Errorf("a late, older snapshot overwrote the newer one: on disk %v", onDisk)
	}
}

// A code has to be on disk before it is handed out. If serve restarts first, the link
// comes back without it, the next open mints a DIFFERENT one, and the code the owner
// already gave someone stops working.
func TestAMintedCodeIsOnDiskBeforeItIsHandedOut(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	m := NewEnrollManager([]EnrolledDevice{{ID: "g1", Token: "tok", Scope: "guest"}},
		func([]EnrolledDevice) { entered <- struct{}{}; <-release })
	handed := make(chan string, 1)
	go func() { code, _ := m.ShareCode("g1"); handed <- code }()

	select {
	case <-entered:
	case <-handed:
		t.Fatal("the code was handed out before its save had even begun")
	case <-time.After(2 * time.Second):
		t.Fatal("minting a code for a link without one saved nothing")
	}
	// The save is held open. A synchronous save means the code cannot have come back yet.
	select {
	case <-handed:
		t.Error("the code was handed out while it was still being written")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if code := <-handed; code == "" {
		t.Error("no code came back")
	}
}
