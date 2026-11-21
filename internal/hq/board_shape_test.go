package hq

import "strings"
import "testing"

import "github.com/chenchaoyi/gtmux/internal/i18n"

func TestRunbookSpecifiesTheBoardShape(t *testing.T) {
	for _, want := range []string{"one compact row per live pane", "newest first", "Keep detail in source records"} {
		if !strings.Contains(playbookText("en", "startup.md"), want) {
			t.Errorf("startup runbook missing %q", want)
		}
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
// old one from its own file while the playbook tells it something else. Since
// hq-first-person the skeleton follows GTMUX_LANG; the Chinese seed keeps the exact
// headings this test always pinned.
func TestBoardSeedHasBothParts(t *testing.T) {
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	i18n.SetLang("zh")
	seed := hqNotesSeeds()["board.md"]
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
	prev := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(prev) })
	i18n.SetLang("zh")
	seed := hqNotesSeeds()["board.md"]
	for _, h := range boardHeadings {
		if !strings.Contains(playbookText("zh", "startup.md"), h) {
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
		if strings.Contains(playbookText("en", "startup.md"), banned) || strings.Contains(hqNotesSeeds()["board.md"], banned) {
			t.Errorf("%q is back — it is what produced 姿态/在飞的线/线", banned)
		}
	}
}
