package hq

import "testing"

// standingDecide is the family rule. Its dangerous direction is SILENCE, so every
// suppression here is paired with the drift that must break it.
func TestStandingDecide(t *testing.T) {
	const now, repeat, floor = int64(10_000_000), int64(1800), int64(12 * 3600)
	knocked := standingState{KnockedAt: now - repeat, Breach: "age", Moved: 5}

	cases := []struct {
		name  string
		w     standingWorld
		st    standingState
		want  standingVerdict
		clock int64
	}{
		{"nothing over the line", standingWorld{}, knocked, standingClear, now},
		{"first knock", standingWorld{Breach: "age"}, standingState{}, standingKnock, now},
		{"inside the minimum spacing", standingWorld{Breach: "age", Moved: 5},
			standingState{KnockedAt: now - 1, Breach: "age", Moved: 5}, standingHold, now},
		{"unchanged world, past the spacing", standingWorld{Breach: "age", Moved: 5},
			knocked, standingHold, now},
		{"breach set changed", standingWorld{Breach: "ctx,age", Moved: 5},
			knocked, standingKnock, now},
		{"world moved", standingWorld{Breach: "age", Moved: 6},
			knocked, standingKnock, now},
		{"safety floor", standingWorld{Breach: "age", Moved: 5},
			standingState{KnockedAt: now - floor, Breach: "age", Moved: 5}, standingKnock, now},
	}
	for _, c := range cases {
		got, _ := standingDecide(c.clock, c.w, c.st, repeat, floor)
		if got != c.want {
			t.Errorf("%s: verdict = %v, want %v", c.name, got, c.want)
		}
	}

	// Clearing forgets the fingerprint, so the NEXT breach gets a fresh first knock
	// rather than inheriting a suppression it never earned.
	_, next := standingDecide(now, standingWorld{}, knocked, repeat, floor)
	if next != (standingState{}) {
		t.Errorf("clearing must forget the last knock, got %+v", next)
	}
	if v, _ := standingDecide(now, standingWorld{Breach: "age"}, next, repeat, floor); v != standingKnock {
		t.Error("a breach after a recovery must knock immediately")
	}

	// floor 0 means "silent until something changes" — the opt-in the config documents.
	if v, _ := standingDecide(now, standingWorld{Breach: "age", Moved: 5},
		standingState{KnockedAt: now - 100*floor, Breach: "age", Moved: 5}, repeat, 0); v != standingHold {
		t.Error("floor 0 must never fire on its own")
	}
}

// A delivered knock records the world it described — otherwise the next comparison is
// against stale facts and the alarm either repeats forever or goes silent forever.
func TestStandingDecide_KnockRecordsItsWorld(t *testing.T) {
	const now, repeat, floor = int64(10_000_000), int64(1800), int64(12 * 3600)
	w := standingWorld{Breach: "ctx,age", Moved: 42}
	v, next := standingDecide(now, w, standingState{KnockedAt: now - repeat, Breach: "age"}, repeat, floor)
	if v != standingKnock {
		t.Fatal("a changed breach set must knock")
	}
	if next.KnockedAt != now || next.Breach != w.Breach || next.Moved != w.Moved {
		t.Fatalf("the knock must record what it announced, got %+v", next)
	}
	// Immediately re-evaluating the same world is now a no-news repeat.
	if v, _ := standingDecide(now+repeat, w, next, repeat, floor); v != standingHold {
		t.Error("re-stating the just-announced world must hold")
	}
}
