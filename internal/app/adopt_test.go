package app

import "testing"

func TestAdoptSessionName(t *testing.T) {
	cases := map[string]string{
		"/Users/x/proj/acme-mobile": "acme-mobile",
		"/Users/x/my.proj":          "my-proj", // '.' → '-'
		"/Users/x/a b":              "a-b",     // space → '-'
		"/tmp/":                     "tmp",     // trailing slash
		"/":                         "",        // nothing usable
		"":                          "",
	}
	for cwd, want := range cases {
		if got := adoptSessionName(cwd); got != want {
			t.Errorf("adoptSessionName(%q) = %q, want %q", cwd, got, want)
		}
	}
}
