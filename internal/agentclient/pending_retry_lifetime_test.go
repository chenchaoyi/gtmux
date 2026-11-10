package agentclient

import (
	"strings"
	"testing"
	"time"
)

func TestMSD011DurablePendingKeepsOneIdentityAcrossRetryAndPrune(t *testing.T) {
	_, a, clock := attachmentFixture(t)
	r, err := Prepare(a, "synthetic", "durable-id", false, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, age := range []time.Duration{21 * time.Second, time.Minute, 48 * time.Hour} {
		retry, e := Prepare(a, "synthetic", "durable-id", false, clock.Now.Add(age))
		if e != nil || retry.ID != r.ID {
			t.Fatalf("pending split at %s: first=%s next=%s err=%v", age, r.ID, retry.ID, e)
		}
	}
	changed := a
	changed.SessionID = "different"
	if _, e := Prepare(changed, "synthetic", "durable-id", false, clock.Now.Add(49*time.Hour)); e == nil {
		t.Fatal("late pending retargeted")
	}
	if _, e := Prepare(a, "changed body", "durable-id", false, clock.Now.Add(49*time.Hour)); e == nil {
		t.Fatal("late pending changed body")
	}
	if err := ClaimNative(r); err != nil {
		t.Fatal(err)
	}
	if err := ClaimNative(r); err == nil {
		t.Fatal("pending claimed twice")
	}
	if _, e := Prepare(a, "synthetic", "durable-id", false, clock.Now.Add(72*time.Hour)); e == nil {
		t.Fatal("unknown claim replayed")
	}
}
func TestMSD011PrunedOrReplacedNativePendingCannotRegainAuthority(t *testing.T) {
	for _, kind := range []string{"settled-pruned", "replaced", "changed-payload"} {
		t.Run(kind, func(t *testing.T) {
			_, a, clock := attachmentFixture(t)
			r, err := Prepare(a, "synthetic", "retire-id", false, clock.Now)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "settled-pruned":
				if err = ClaimNative(r); err != nil {
					t.Fatal(err)
				}
				if err = FinishNative(r, "accepted", ""); err != nil {
					t.Fatal(err)
				}
				next, e := Prepare(a, "synthetic", "retire-id", false, clock.Now.Add(48*time.Hour))
				if e != nil || next.ID == r.ID {
					t.Fatal("settled retention changed", e)
				}
			case "replaced":
				other := r
				other.Instance = strings.Repeat("f", 32)
				putJSON(t, InputDir()+"/"+r.ID+".pending.json", other)
			case "changed-payload":
				r.Text = "changed"
				r.TextHash = "changed"
			}
			if err = ClaimNative(r); err == nil {
				t.Fatal("stale or altered request regained mutation authority", kind)
			}
		})
	}
}
