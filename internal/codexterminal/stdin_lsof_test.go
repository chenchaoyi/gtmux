package codexterminal

import "testing"

func TestWrappedCodexStdinDescriptor(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"p300\nf0\nn/dev/ttys001\n", "ttys001"},
		{"p300\nf0\nn/dev/pts/1\n", "pts/1"},
		{"p300\nf0\nnpipe\n", ""},
		{"p300\nf1\nn/dev/ttys001\n", ""},
		{"p301\nf0\nn/dev/ttys001\n", ""},
		{"p300\nf0\nn/dev/ttys001\np301\nf0\nn/dev/ttys001\n", ""},
		{"p300\nf0\nn/dev/ttys001\nn/dev/ttys002\n", ""},
		{"f0\nn/dev/ttys001\n", ""},
		{"p300\nf0\nn/tmp/ttys001\n", ""},
	} {
		if got := stdinFromLsof(tc.raw, 300); got != tc.want {
			t.Errorf("stdin descriptor %q = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
