package app

import "testing"

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestOrderByTabOrder: recorded order wins; sessions not in the record keep their
// order at the end; sessions in the record but no longer live are skipped; no
// record leaves the input unchanged.
func TestOrderByTabOrder(t *testing.T) {
	if got := orderByTabOrder([]string{"A", "B", "C"}, []string{"C", "A", "D"}); !eqStrs(got, []string{"C", "A", "B"}) {
		t.Errorf("reorder = %v, want [C A B]", got)
	}
	in := []string{"A", "B"}
	if got := orderByTabOrder(in, nil); !eqStrs(got, in) {
		t.Errorf("no record = %v, want unchanged %v", got, in)
	}
	if got := orderByTabOrder([]string{"X", "Y"}, []string{"Z", "Y", "X"}); !eqStrs(got, []string{"Y", "X"}) {
		t.Errorf("dead record entries skipped = %v, want [Y X]", got)
	}
}
