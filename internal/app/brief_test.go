package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The colour rule is the whole point of this command, so it is the first thing tested:
// RED IS FOR DECISIONS. A machine number that is permanently red is a line nobody reads,
// and this machine sits near 90% disk permanently.
func TestBriefColorDiscipline(t *testing.T) {
	if got := briefHeadColor(briefDecision); got != i18n.Red {
		t.Errorf("a decision must be red, got %q", got)
	}
	for _, g := range []string{briefAttention, briefLedger, "", "something-new"} {
		if briefHeadColor(g) == i18n.Red {
			t.Errorf("grade %q must not be red — red is reserved for a decision", g)
		}
	}
	// A machine number is dim even inside a decision-grade brief: the brief may be
	// urgent while that particular line is only context.
	out := renderBrief(briefInput{
		Grade: briefDecision, Headline: "two sessions need you",
		Lines: []briefLine{{Label: "disk", Value: "41GB free", Tone: briefDim}},
	}, true)
	i := strings.Index(out, "41GB free")
	if i < 0 {
		t.Fatal("the value went missing")
	}
	if !strings.Contains(out[:i], i18n.Dim) {
		t.Error("a dim-toned line must render dim, not compete with the decision")
	}
	if strings.Contains(out[i-len(i18n.Red)-4:i], i18n.Red) {
		t.Error("a machine number must never be red")
	}
}

// An unknown grade reads as the QUIETEST one. A brief that cannot say how loud it is has
// not earned loud — the safe direction on a channel whose value is scarcity.
func TestBriefUnknownGradeIsQuiet(t *testing.T) {
	if briefGlyph("wat") != briefGlyph(briefLedger) {
		t.Error("an unknown grade must fall to the ledger register")
	}
}

// Alignment is measured in DISPLAY CELLS. Padding by rune or byte count ragged the
// column exactly where this project's users write — a CJK label is two cells per rune.
func TestBriefAlignsByDisplayWidth(t *testing.T) {
	out := renderBrief(briefInput{
		Headline: "h",
		Lines: []briefLine{
			{Label: "磁盘", Value: "A"},  // 4 cells
			{Label: "api", Value: "B"}, // 3 cells
			{Label: "x", Value: "C"},   // 1 cell
		},
	}, false)
	var cols []int
	for _, line := range strings.Split(out, "\r\n") {
		for _, v := range []string{"A", "B", "C"} {
			if strings.HasSuffix(strings.TrimRight(line, " "), v) {
				cols = append(cols, i18n.DispWidth(line[:strings.LastIndex(line, v)]))
			}
		}
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 value columns, got %d (%v)", len(cols), cols)
	}
	for _, c := range cols[1:] {
		if c != cols[0] {
			t.Errorf("values start at differing columns %v — the column is ragged", cols)
		}
	}
}

// Every brief carries the marker, so HQ can recognise its OWN voice in its scrollback.
// Without it the report becomes input to the next screen read — the self-feeding loop
// the wake channel already had to solve by excluding HQ's own records.
func TestBriefCarriesItsMarker(t *testing.T) {
	out := renderBrief(briefInput{Grade: briefLedger, Headline: "quiet"}, false)
	if !strings.Contains(out, briefMarker) {
		t.Fatalf("a brief must be recognisable as one, got %q", out)
	}
}

// Lines end CR+LF: the pane's tty is in raw-ish mode with a TUI on it, and a bare \n
// leaves the cursor mid-row so the next line starts staircased.
func TestBriefLinesAreCRLFTerminated(t *testing.T) {
	out := renderBrief(briefInput{Headline: "h", Lines: []briefLine{{Label: "a", Value: "b"}}}, false)
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Error("a bare \\n would staircase the output on a raw tty")
	}
}

// Colour off must produce the SAME text, so a test can reason about layout and a
// non-tty consumer gets clean bytes.
func TestBriefColorOffIsPlain(t *testing.T) {
	in := briefInput{Grade: briefDecision, Headline: "h", Lines: []briefLine{{Label: "a", Value: "b", Tone: briefDecision}}}
	if strings.Contains(renderBrief(in, false), "\033") {
		t.Error("colour off must emit no escapes")
	}
}
