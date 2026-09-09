package hq

import (
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
