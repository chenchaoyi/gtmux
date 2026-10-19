package app

import (
	"bytes"
	"testing"
)

func TestDSTProfilePreservesForeignTextAndIsReversible(t *testing.T) {
	original := []byte("# user settings\n- id: foreign\n  config: !!js process.env.SYNTHETIC\n")
	block := "- insert:\n    - id: gtmux-native-dsh-terminal\n      name: file:///synthetic.mjs\n"
	next, err := dstProfileContent(original, block)
	if err != nil || !bytes.HasPrefix(next, original) {
		t.Fatalf("foreign text changed %s %v", next, err)
	}
	again, err := dstProfileContent(next, block)
	if err != nil || !bytes.Equal(again, next) {
		t.Fatal("not idempotent")
	}
	removed, err := dstProfileContent(next, "")
	if err != nil || !bytes.Equal(removed, original) {
		t.Fatalf("not reversible %q %v", removed, err)
	}
	for _, bad := range []string{"{}", "[{}]", "[]\n---\n[]", dstProfileBegin + "[]\n"} {
		if _, err := dstProfileContent([]byte(bad), block); err == nil {
			t.Fatalf("accepted invalid/ambiguous profile %q", bad)
		}
	}
	for _, base := range []string{"[]\n", "# user comment\n[]\n", ""} {
		if _, err := dstProfileContent([]byte(base), block); err != nil {
			t.Fatal(err)
		}
	}
}
