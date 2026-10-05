package dispatch

import "testing"

func TestDraftIsExactly(t *testing.T) {
	payload := "» ◆ gtmux·waiting  %14 needs a decision · #ab12cd"
	for draft, want := range map[string]bool{
		payload: true,
		"» ◆ gtmux·waiting  %14 needs a\n    decision · #ab12cd  ": true, // re-wrapped, indented, padded
		"» ◆ gtmux·waiting %14 needsa decision·#ab12cd":            true, // spacing only
		payload + " ok": false,
		"ok " + payload: false,
		"» ◆ gtmux·waiting  %14 needs a choice · #ab12cd": false,
		"» ◆ gtmux·waiting  %14 needs a decision":         false, // half-rendered
		"": false,
	} {
		if got := DraftIsExactly(draft, payload); got != want {
			t.Errorf("DraftIsExactly(%q) = %v, want %v", draft, got, want)
		}
	}
	if DraftIsExactly("", "  ") {
		t.Error("an empty payload must never match")
	}
	// The extraction strips a border glyph from either edge of a row: the payload's own
	// " │ " is lost when the wrap puts it there, and only there.
	sep := "» ◆ gtmux·waiting (%5) │ 需要你决定 · #ab12cd"
	for draft, want := range map[string]bool{
		"» ◆ gtmux·waiting (%5)\n需要你决定 · #ab12cd":     true,  // │ ended row 1 or began row 2
		"» ◆ gtmux·waiting\n(%5) 需要你决定 · #ab12cd":     false, // │ gone from mid-row
		"» ◆ gtmux·waiting (%5) 需要你决定 · #ab12cd":      false, // │ deleted
		"» ◆ gtmux·waiting │ (%5) │ 需要你决定 · #ab12cd":  false, // │ added mid-row
		"» ◆ gtmux·waiting (%5) │ 需要你 > 决定 · #ab12cd": false, // > added mid-row
		"» ◆ gtmux·waiting (%5)\n\n需要你决定 · #ab12cd":   true,  // a blank row
		"» ◆ gtmux·waiting (%5)\n需要你决定 · #ab12cd\n继续": false,
	} {
		if got := DraftIsExactly(draft, sep); got != want {
			t.Errorf("DraftIsExactly(%q) = %v, want %v", draft, got, want)
		}
	}
}
