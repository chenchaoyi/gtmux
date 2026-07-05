package app

import "testing"

func TestAdoptSessionName(t *testing.T) {
	cases := map[string]string{
		"/Users/ccy/meituan/chenchaoyi/diting-mobile": "diting-mobile",
		"/Users/ccy/my.proj":                          "my-proj", // '.' → '-'
		"/Users/ccy/a b":                              "a-b",     // space → '-'
		"/tmp/":                                       "tmp",     // trailing slash
		"/":                                           "",        // nothing usable
		"":                                            "",
	}
	for cwd, want := range cases {
		if got := adoptSessionName(cwd); got != want {
			t.Errorf("adoptSessionName(%q) = %q, want %q", cwd, got, want)
		}
	}
}
