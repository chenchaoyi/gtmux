package codexbridge

import (
	"context"
	"testing"
	"time"
)

func TestCFR004OnlySuccessfulNewerConnectionChangesAuthority(t *testing.T) {
	var epoch connectionEpoch
	a := epoch.reserve()
	if epoch.current(a) || !epoch.activate(a) || !epoch.current(a) {
		t.Fatal("unupgraded attempt had authority, or successful upgrade was lost")
	}
	b := epoch.reserve() // refused or still pending: no activation
	if !epoch.current(a) || epoch.current(b) {
		t.Fatal("failed handshake changed incumbent authority")
	}
	c := epoch.reserve()
	if !epoch.activate(c) || !epoch.current(c) || epoch.current(a) {
		t.Fatal("successful newer connection did not retire only the incumbent")
	}
	if epoch.activate(b) || !epoch.current(c) {
		t.Fatal("late older handshake reclaimed authority")
	}
	if epoch.activate(0) || epoch.activate(c) || epoch.activate(c+1) || !epoch.current(c) {
		t.Fatal("invalid or unreserved activation changed authority")
	}
}

func TestCFR004UpgradeRetainsOnlyTheRemainingAdmissionBudget(t *testing.T) {
	for _, ctx := range []context.Context{context.Background(), func() context.Context { c, cancel := context.WithCancel(context.Background()); cancel(); return c }()} {
		if _, err := admissionUpgradeTimeout(ctx); err == nil {
			t.Fatal("unbounded or cancelled handshake was admitted")
		}
	}
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := admissionUpgradeTimeout(expired); err == nil {
		t.Fatal("expired handshake became unbounded")
	}
	live, cancelLive := context.WithTimeout(context.Background(), admissionBudget)
	defer cancelLive()
	d, err := admissionUpgradeTimeout(live)
	if err != nil || d <= 0 || d > admissionBudget {
		t.Fatal("upgrade received a new or invalid budget", d, err)
	}
}
