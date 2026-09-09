package hq

import (
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A board cell that has become an investigation log.
//
// The rule is in the charter now, but a rule nothing measures is a rule nobody keeps —
// which is why `gtmux doctor` reports this. These pin the measurement: the unit is
// CHARACTERS (the board is usually Chinese, where bytes would flag an ordinary sentence),
// and a pasted capture inside a fence is evidence, not prose.
func writeBoard(t *testing.T, body string) {
	t.Helper()
	// HOME, not XDG_CONFIG_HOME: nothing in gtmux reads the XDG variables, so setting
	// one only LOOKS like a redirect. This test overwrote the operator's live board on
	// 2026-09-09 for exactly that reason.
	t.Setenv("HOME", t.TempDir())
	p := BoardPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOversizeBoardCellsCountsCharactersNotBytes(t *testing.T) {
	// 300 Chinese characters is 900 bytes. A byte count would call this oversize; it is
	// an ordinary long sentence and must pass.
	writeBoard(t, "# 板\n\n"+strings.Repeat("态", 300)+"\n")
	if n, longest := OversizeBoardCells(); n != 0 {
		t.Errorf("300 Chinese characters flagged as oversize (longest=%d) — that is a byte count", longest)
	}
}

func TestOversizeBoardCellsFindsTheWall(t *testing.T) {
	writeBoard(t, "# 板\n\n短的一行\n\n"+strings.Repeat("态", BoardCellBudget+80)+"\n")
	n, longest := OversizeBoardCells()
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	if longest != BoardCellBudget+80 {
		t.Errorf("longest = %d, want %d", longest, BoardCellBudget+80)
	}
}

func TestAPastedCaptureIsEvidenceNotProse(t *testing.T) {
	// Shortening a fenced capture would lose the thing it was pasted to show.
	writeBoard(t, "# 板\n\n```\n"+strings.Repeat("x", BoardCellBudget+200)+"\n```\n")
	if n, _ := OversizeBoardCells(); n != 0 {
		t.Errorf("a fenced capture was counted as an overlong cell")
	}
}

func TestNoBoardIsNotAComplaint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if n, longest := OversizeBoardCells(); n != 0 || longest != 0 {
		t.Errorf("got %d/%d for a board that does not exist", n, longest)
	}
}

// The one heading gtmux owns, and the three places that must agree on it.
//
// Both surfaces LIFT this section to the top of the board, and a surface can only lift a
// section it can recognise. The name therefore stops being HQ's to choose — and it now
// lives in three languages' source, which is exactly the shape that drifts. So the Go
// seed is the source and the other two are checked against it.
func TestTheLiftedHeadingIsSpelledTheSameOnAllThreeSurfaces(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, f := range []string{
		filepath.Join("macapp", "Sources", "GtmuxBar", "BoardOutline.swift"),
		filepath.Join("mobileapp", "src", "screens", "boardSections.ts"),
	} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, want := range []string{AskHeadingEN, AskHeadingZH} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s does not carry %q — that surface cannot lift the section, and it degrades to showing nothing without saying why", f, want)
			}
		}
	}
}

func TestTheSeededBoardCarriesTheLiftedSection(t *testing.T) {
	// A seeded board must contain the heading the surfaces look for, or a fresh HQ home
	// has a band that can never appear.
	t.Setenv("HOME", t.TempDir())
	for _, lang := range []string{"en", "zh"} {
		prev := i18n.Lang()
		i18n.SetLang(lang)
		seed := boardSeed()
		i18n.SetLang(prev)
		if !IsAskHeading(pickAskHeading(seed)) {
			t.Errorf("the %s seed has no heading the surfaces would lift:\n%s", lang, seed)
		}
	}
}

// pickAskHeading returns the seed's third-level heading, whatever language it is in.
func pickAskHeading(seed string) string {
	for _, l := range strings.Split(seed, "\n") {
		if strings.HasPrefix(l, "### ") {
			return l
		}
	}
	return ""
}

func TestAnEmptySectionIsTheNormalState(t *testing.T) {
	// The charter says empty is normal and an empty section shows no band. This pins the
	// recogniser's half of that: a heading is a heading whatever level it sits at, so the
	// surfaces find it under ① as the seed writes it.
	for _, h := range []string{"### " + AskHeadingZH, "## " + AskHeadingEN, AskHeadingZH, "  ### " + AskHeadingEN + "  "} {
		if !IsAskHeading(h) {
			t.Errorf("IsAskHeading(%q) = false", h)
		}
	}
	for _, h := range []string{"### 现状", "### Still waiting", "", "###"} {
		if IsAskHeading(h) {
			t.Errorf("IsAskHeading(%q) = true — that is not the section", h)
		}
	}
}
