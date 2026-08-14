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
	if hqPlaybookVersion < 22 {
		t.Errorf("hqPlaybookVersion = %d; the board's Chinese headings shipped in 22", hqPlaybookVersion)
	}
}

// boardHeadings are the words the board's two parts are CALLED — fixed, and fixed in
// Chinese, because the commander reads this on a phone in Chinese.
//
// They are given rather than translated for a measured reason: the playbook used to name
// the parts only in English ("Posture", one row per "ship"), leaving HQ to render them
// itself, and it produced `① 姿态 — 当前在飞的线` over a column called `线` — word-for-word
// renderings of posture/in-flight/ship that no Chinese speaker would write. Naming a thing
// in one language and hoping for the other is how a product ends up sounding translated.
var boardHeadings = []string{"① 现状", "② 交接记录"}

// The seed a FRESH home starts from must already have the shape, or a new HQ learns the
// old one from its own file while the playbook tells it something else.
func TestBoardSeedHasBothParts(t *testing.T) {
	seed := hqNotesSeeds["board.md"]
	for _, want := range append(boardHeadings, "新的在最上面") {
		if !strings.Contains(seed, want) {
			t.Errorf("board seed missing %q", want)
		}
	}
	// The table itself must survive — it is the part that had vanished.
	if !strings.Contains(seed, "| pane | 在做什么 |") {
		t.Error("board seed lost the current-state table")
	}
}

// The playbook and the seed must agree on the headings. They are two copies of one
// decision, and when they drift a fresh home reads one shape out of its own file while
// being told another by its charter — with no error anywhere.
func TestPlaybookAndSeedAgreeOnTheHeadings(t *testing.T) {
	seed := hqNotesSeeds["board.md"]
	for _, h := range boardHeadings {
		if !strings.Contains(hqInstructions, h) {
			t.Errorf("playbook does not name the part %q", h)
		}
		if !strings.Contains(seed, h) {
			t.Errorf("seed does not name the part %q", h)
		}
	}
}

// The words that got this rewritten must not come back.
func TestBoardDoesNotSpeakTranslationese(t *testing.T) {
	for _, banned := range []string{"① Posture", "per LIVE ship", "ship (loc/pane)"} {
		if strings.Contains(hqInstructions, banned) || strings.Contains(hqNotesSeeds["board.md"], banned) {
			t.Errorf("%q is back — it is what produced 姿态/在飞的线/线", banned)
		}
	}
}
