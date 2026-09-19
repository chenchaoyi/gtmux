package transcript

import "testing"

// The shape Claude Code 2.1.277 records for a pasted prompt, as found in an HQ transcript
// on 2026-09-19: a wake batch whose quoted field spans two lines, wrapped in tags whose
// closing half also carries the id.
const wrappedWake = "\n\n<pasted_content id=\"676c\">\n» ◆ gtmux·goal-changed  web:0.0 (%20) │ goal:\"fix the chat view\n/tmp/shot.png\" · #81a11d\n</pasted_content id=\"676c\">\n"

// A wake batch is gtmux's own traffic. Wrapped or not, it must reduce to nothing: the
// phone's HQ chat showed the empty tag pair as the user's message when it did not.
func TestAWrappedWakeBatchIsStillNotAUserPrompt(t *testing.T) {
	if text, kind := ClassifyUserPrompt(wrappedWake); kind != PromptDrop {
		t.Errorf("a wrapped wake batch classified as %q with text %q, want drop", kind, text)
	}
}

// A person's own paste keeps its words and loses the tags.
func TestAWrappedHumanPasteKeepsItsTextWithoutTags(t *testing.T) {
	in := "<pasted_content id=\"a1b2\">\nplease look at the failing test\nand fix it\n</pasted_content id=\"a1b2\">"
	text, kind := ClassifyUserPrompt(in)
	if kind != PromptUser || text != "please look at the failing test\nand fix it" {
		t.Errorf("got %q (%s), want the pasted words without tags", text, kind)
	}
}

func TestUnwrapPastedLeavesUnwrappedTextAlone(t *testing.T) {
	for _, s := range []string{"plain words", "<b>not our tag</b>", ""} {
		if got := UnwrapPasted(s); got != s {
			t.Errorf("UnwrapPasted(%q) = %q, want it unchanged", s, got)
		}
	}
}
