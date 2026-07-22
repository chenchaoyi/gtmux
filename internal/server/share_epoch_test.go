package server

import "testing"

// Pane grants are only meaningful inside ONE tmux server lifetime: after a restart the
// ids are reassigned, so a stored "%17" can address a completely different pane. These
// pin the fail-closed contract (share-grant-epoch).

func TestGrantsStaleAfterTmuxRestart(t *testing.T) {
	saved := ShareState{}
	m := NewShareManager(ShareState{Enabled: true, Panes: []string{"%17"}, ViewPanes: []string{"%17"}},
		func(s ShareState) { saved = s })

	epoch := "111@Mon Jul 21 10:00:00 2026"
	m.SetEpochSource(func() string { return epoch })

	// Grants predate any stamp → they cannot be proven to belong to this server.
	if !m.GrantsStale() {
		t.Fatal("unstamped grants must be stale while a tmux server is running")
	}

	// The owner (re)grants against the running server.
	m.StampEpoch()
	if m.GrantsStale() {
		t.Error("freshly stamped grants must not be stale")
	}
	if saved.PaneEpoch != epoch {
		t.Errorf("persisted PaneEpoch = %q; want %q (must survive a serve restart)", saved.PaneEpoch, epoch)
	}

	// tmux restarts → a new identity → every stored pane id is suspect again.
	epoch = "222@Tue Jul 22 09:00:00 2026"
	if !m.GrantsStale() {
		t.Error("after the tmux server changed, grants MUST be stale (ids were reassigned)")
	}
}

func TestGrantsNotStaleWhenPersistedEpochMatches(t *testing.T) {
	const epoch = "333@Wed Jul 22 08:00:00 2026"
	// A serve restart reloads the stamp from disk; the tmux server is the same one.
	m := NewShareManager(ShareState{Panes: []string{"%17"}, PaneEpoch: epoch}, func(ShareState) {})
	m.SetEpochSource(func() string { return epoch })
	if m.GrantsStale() {
		t.Error("grants stamped against the SAME running tmux server must remain valid across a serve restart")
	}
}

// Never lock out a setup we can't reason about: no tmux server (or an unreadable
// identity) means there are no panes to serve anyway.
func TestGrantsNotStaleWithoutATmuxIdentity(t *testing.T) {
	m := NewShareManager(ShareState{Panes: []string{"%17"}, PaneEpoch: "old"}, func(ShareState) {})
	if m.GrantsStale() {
		t.Error("with no epoch source at all, nothing should be considered stale")
	}
	m.SetEpochSource(func() string { return "" })
	if m.GrantsStale() {
		t.Error("an unknown tmux identity must not invalidate grants")
	}
}

// A guest whose grants are stale must see NOTHING — the radar can't leak a pane the
// owner never shared.
func TestGuestRadarIsEmptyWhenGrantsAreStale(t *testing.T) {
	raw := []byte(`[{"pane_id":"%17","task":"secret"},{"pane_id":"%20","task":"other"}]`)
	dev := EnrolledDevice{ViewPanes: []string{"%17"}}
	if got := string(filterAgentsForGuest(raw, dev, true)); got != "[]" {
		t.Errorf("stale guest radar = %s; want [] (fail closed)", got)
	}
	// and it still works normally when the grants are current
	if got := string(filterAgentsForGuest(raw, dev, false)); got == "[]" {
		t.Error("a current grant must still show its allowed pane")
	}
}
