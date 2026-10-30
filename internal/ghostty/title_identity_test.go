package ghostty

import "testing"

func TestLastTabKeepsExactTrailingWhitespace(t *testing.T) {
	for _, title := range []string{"日常更新 — tmp %32 ", "work — split  ", "  work — shell\t"} {
		for _, ending := range []string{"\n", "\r\n", "\n\n"} {
			output := []byte("1\t1\tother — first \n1\t2\t" + title + ending)
			rows := parseTabTitles(scriptOutput(output))
			if len(rows) != 2 || rows[1].name != title {
				t.Fatalf("last tab identity changed: %q -> %#v", title, rows)
			}
			if rows[0].name != "other — first " {
				t.Fatal("preceding tab title changed")
			}
		}
	}
}

func TestScriptResultTokensKeepExistingContract(t *testing.T) {
	for _, token := range []string{"ok", "notfound", ""} {
		if got := scriptOutput([]byte(token + "\n")); got != token {
			t.Fatalf("%q: %q", token, got)
		}
	}
}
