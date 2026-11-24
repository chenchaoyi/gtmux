package userservice

import "testing"

func TestGSS018LaunchdRejectsTruncatedDisabledTable(t *testing.T) {
	const head = "disabled services = {\n"
	const label = "com.gtmux.serve"
	const disabled = "\t\"com.gtmux.serve\" => disabled\n"
	for _, tc := range []struct {
		text string
		want bool
	}{
		{head + disabled + "}\n", true},
		{disabled, false},
		{head + disabled, false},
		{head + disabled + disabled + "}\n", false},
		{head + disabled + "}\n" + head, false},
	} {
		if got := persistentlyDisabled([]byte(tc.text), label); got != tc.want {
			t.Fatalf("disabled table result for %q: %v, want %v", tc.text, got, tc.want)
		}
	}
}
