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

// TestShrinksTabOrder guards the post-reboot clobber: a degraded snapshot (only a
// bootstrap 'main', or an empty AppleScript read) must NOT overwrite the richer
// recorded order, but any genuine change (new session) must go through.
func TestShrinksTabOrder(t *testing.T) {
	full := []string{"日常更新", "Diting", "ccy-workspace", "Hammer", "main"}
	cases := []struct {
		name       string
		next, prev []string
		want       bool
	}{
		{"post-reboot main-only clobbers full", []string{"main"}, full, true},
		{"applescript-failed empty clobbers full", nil, full, true},
		{"pure removal", []string{"Diting", "main"}, full, true},
		{"unchanged", full, full, false},
		{"reorder, same set", []string{"main", "Hammer", "Diting", "ccy-workspace", "日常更新"}, full, false},
		{"a new session appeared", []string{"main", "NewProj"}, full, false},
		{"grew", append([]string{"NewProj"}, full...), full, false},
		{"no prior record", full, nil, false},
	}
	for _, tc := range cases {
		if got := shrinksTabOrder(tc.next, tc.prev); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
