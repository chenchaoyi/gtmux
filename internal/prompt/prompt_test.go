package prompt

import (
	"reflect"
	"testing"
)

func TestParseOptions_ClaudeBox(t *testing.T) {
	text := `╭───────────────────────────────────────────╮
│ Do you want to make this edit to serve.go?  │
│                                             │
│ ❯ 1. Yes                                    │
│   2. Yes, and don't ask again this session  │
│   3. No, and tell Claude what to do (esc)   │
╰───────────────────────────────────────────╯`
	got := ParseOptions(text)
	want := []Option{
		{1, "Yes"},
		{2, "Yes, and don't ask again this session"},
		{3, "No, and tell Claude what to do (esc)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_PlainAndSelectorVariants(t *testing.T) {
	text := "Continue?\n  1. Yes\n> 2. No"
	got := ParseOptions(text)
	want := []Option{{1, "Yes"}, {2, "No"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_LatestMenuWins(t *testing.T) {
	// An older menu scrolled up, a fresh one below — the fresh run (restart at 1)
	// must win, not a concatenation.
	text := "1. old-a\n2. old-b\n... lots of output ...\n1. new-a\n2. new-b\n3. new-c"
	got := ParseOptions(text)
	want := []Option{{1, "new-a"}, {2, "new-b"}, {3, "new-c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsANSIColors(t *testing.T) {
	// A colored capture (capture-pane -e) leaks SGR codes into the menu; the ESC
	// byte is non-printing so users saw the bare "[38;5;153m…[0m" in the option
	// labels (reported 2026-06-29). The parser must strip them.
	esc := "\x1b"
	text := "❯ 1. " + esc + "[38;5;153mgtmux update" + esc + "[39m 之后" + esc + "[1m 自动" + esc + "[0m\n" +
		"  2. " + esc + "[31mNo" + esc + "[0m, keep current"
	got := ParseOptions(text)
	want := []Option{
		{1, "gtmux update 之后 自动"},
		{2, "No, keep current"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_StripsOSCHyperlinks(t *testing.T) {
	// Claude Code wraps file paths in OSC 8 hyperlinks; strip them too.
	esc := "\x1b"
	bel := "\x07"
	text := "1. open " + esc + "]8;;file:///tmp/a.go" + bel + "a.go" + esc + "]8;;" + bel + " now"
	got := ParseOptions(text)
	want := []Option{{1, "open a.go now"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestParseOptions_None(t *testing.T) {
	if got := ParseOptions("just some logs\nno menu here\nworking…"); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestParseOptions_GapBreaksRun(t *testing.T) {
	// "3." without a preceding 2. must not attach to option 1.
	text := "1. only\nsome noise\n3. orphan"
	got := ParseOptions(text)
	want := []Option{{1, "only"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
