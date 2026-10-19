package terminalkeys

import "testing"

func TestCodexQuestionFooter(t *testing.T) {
	footer := "Working\n\n? 1 question\n  ⇧← to answer\n\n› Ask Codex to do anything\nGPT-6"
	if a := CodexQuestions("codex", footer, "pane"); len(a) != 1 || a[0].Count != 1 {
		t.Fatalf("live footer: %+v", a)
	}
	for _, screen := range []string{
		"quoted ? 1 question\n⇧← to answer\n› composer", "? 1 question\nCtrl+X to answer\n› composer",
		"? 1 question\n⇧← to answer\nno composer", "? 0 questions\n⇧← to answer\n› composer",
	} {
		if a := CodexQuestions("codex", screen, "pane"); len(a) != 0 {
			t.Errorf("invented action from %q", screen)
		}
	}
	if len(CodexQuestions("dsh", footer, "pane")) != 0 {
		t.Fatal("action leaked to other harness")
	}
	a := CodexQuestions("codex", footer, "one")
	b := CodexQuestions("codex", footer, "two")
	if a[0].Revision == b[0].Revision {
		t.Fatal("action revision lost target binding")
	}
}
