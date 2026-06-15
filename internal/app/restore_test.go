package app

import "testing"

func TestSplitAttached(t *testing.T) {
	cases := []struct {
		line     string
		wantAtt  string
		wantName string
		wantOK   bool
	}{
		{"0 work", "0", "work", true},
		{"1 my session", "1", "my session", true}, // name may contain spaces
		{"0 a b c", "0", "a b c", true},
		{"noSpace", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		att, name, ok := splitAttached(c.line)
		if att != c.wantAtt || name != c.wantName || ok != c.wantOK {
			t.Errorf("splitAttached(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.line, att, name, ok, c.wantAtt, c.wantName, c.wantOK)
		}
	}
}

func TestPaneIDRe(t *testing.T) {
	match := []string{"%0", "%12", "%3.left"}
	for _, s := range match {
		if !paneIDRe.MatchString(s) {
			t.Errorf("paneIDRe should match %q", s)
		}
	}
	noMatch := []string{"work", "session", "1%", ""}
	for _, s := range noMatch {
		if paneIDRe.MatchString(s) {
			t.Errorf("paneIDRe should NOT match %q", s)
		}
	}
}
