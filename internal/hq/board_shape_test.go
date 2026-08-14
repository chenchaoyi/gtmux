package hq

import "strings"
import "testing"

// The board was specified as ONE thing — a pruned, current-state table — so nothing ever
// said which way a dated log inside it should run. HQ wrote one anyway, and a real board
// reached 1078 lines with its top half descending (newest prepended), its bottom half
// ascending (newest appended), the actual newest entry at the very END, and the posture
// table gone entirely. On a phone that reads as one wall.
//
// These pin the instructions that fix it, because the failure is silent: nothing breaks,
// the board just stops being readable, and only a human looking at it notices.
func TestPlaybookSpecifiesTheBoardShape(t *testing.T) {
	for _, want := range []string{
		"TWO PARTS",    // the split is stated, not implied
		"NEWEST FIRST", // the order the log runs
		"PREPEND",      // and how to keep it that way
		"PRUNE",        // the posture table stays current, not cumulative
		"EMPHASIS IS FOR EXCEPTIONS",
	} {
		if !strings.Contains(hqInstructions, want) {
			t.Errorf("playbook must teach %q — a board with no stated order grew two of them", want)
		}
	}
}

// A rule that never reaches an existing HQ home is not a rule: `gtmux hq` only
// regenerates AGENTS.md when the SHIPPED version is newer than the one on disk.
func TestBoardShapeShippedWithAVersionBump(t *testing.T) {
	if hqPlaybookVersion < 21 {
		t.Errorf("hqPlaybookVersion = %d; the two-part board shape shipped in 21", hqPlaybookVersion)
	}
}

// The seed a FRESH home starts from must already have the shape, or a new HQ learns the
// old one from its own file while the playbook tells it something else.
func TestBoardSeedHasBothParts(t *testing.T) {
	seed := hqNotesSeeds["board.md"]
	for _, want := range []string{"① Posture", "② Handoff log", "NEWEST FIRST"} {
		if !strings.Contains(seed, want) {
			t.Errorf("board seed missing %q", want)
		}
	}
	// The posture table itself must survive — it is the part that had vanished.
	if !strings.Contains(seed, "| ship (loc/pane) |") {
		t.Error("board seed lost the posture table")
	}
}
