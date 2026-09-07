package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The window is the denominator of every ctx figure, and gtmux cannot read it: Claude's
// log records tokens and never the window. The old guess — smallest tier at or above the
// tokens seen SO FAR — is wrong in one direction, systematically: a session on a 1M model
// reads against 200k until it grows past 200k.
//
// Measured: HQ reported at ctx 83% holding ~167k tokens of a model whose own log shows
// 999,833 in one request. The truth was 17%, and HQ was told to rotate thirteen times in a
// day because of it.
func TestWindowUsesEvidenceNotJustTheCurrentSize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// A fresh session on a big model, before anything has been seen: the tier guess is
	// all there is, and it is wrong. This is the state that has to heal, not a promise.
	if got := windowFor("claude", "claude-opus-5", 167_000, 0); got != 200_000 {
		t.Errorf("with no evidence the tier still applies, got %d", got)
	}

	// One request on this machine held 999,833 tokens. A window is never smaller than
	// something that fit in it.
	noteWindowEvidence("claude", "claude-opus-5", 999_833)

	if got := windowFor("claude", "claude-opus-5", 167_000, 0); got != 1_000_000 {
		t.Errorf("windowFor = %d, want 1,000,000 — the 83%%-that-was-really-17%% bug", got)
	}
	// And the figure that follows from it.
	if frac := float64(167_000) / float64(windowFor("claude", "claude-opus-5", 167_000, 0)); frac > 0.2 {
		t.Errorf("ctx reads %.0f%%, want ~17%%", frac*100)
	}
}

func TestEvidenceIsPerModelSoASmallOneIsNotInflated(t *testing.T) {
	// The fix must not fail the other way: a genuinely 200k model must keep warning.
	t.Setenv("HOME", t.TempDir())
	noteWindowEvidence("claude", "claude-opus-5", 999_833)
	if got := windowFor("claude", "some-200k-model", 190_000, 0); got != 200_000 {
		t.Errorf("a different model was judged against another model's window: %d", got)
	}
	if got := windowFor("codex", "claude-opus-5", 190_000, 0); got != 200_000 {
		t.Errorf("evidence leaked across agents: %d", got)
	}
}

func TestStatedWindowStillWins(t *testing.T) {
	// Codex states its own window. Evidence is for the agents that do not.
	t.Setenv("HOME", t.TempDir())
	noteWindowEvidence("codex", "gpt-x", 999_833)
	if got := windowFor("codex", "gpt-x", 10_000, 272_000); got != 272_000 {
		t.Errorf("windowFor = %d, want the stated 272,000", got)
	}
}

func TestEvidenceOnlyEverRises(t *testing.T) {
	// A window cannot shrink because a later session happened to be small, and a
	// high-water that could fall would re-open the bug on the next quiet session.
	t.Setenv("HOME", t.TempDir())
	noteWindowEvidence("claude", "m", 999_833)
	noteWindowEvidence("claude", "m", 1_000)
	if got := evidencedWindow("claude", "m"); got != 999_833 {
		t.Errorf("the mark fell to %d", got)
	}
}

func TestNoModelNameIsNotEvidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	noteWindowEvidence("claude", "", 999_833)
	if got := evidencedWindow("claude", ""); got != 0 {
		t.Errorf("recorded evidence against an empty model name: %d", got)
	}
	// And an unreadable store is "no evidence", never a crash.
	if got := evidencedWindow("claude", "never-seen"); got != 0 {
		t.Errorf("invented evidence: %d", got)
	}
}

// Evidence gathered only from LIVE sessions has a hole it cannot climb out of, and HQ fell
// in it: its model is judged against 200k, so at 150k it is told to rotate — which means no
// session of that model ever reaches the size that would prove the window is bigger. The
// false alarm prevents the evidence that would silence it. The history breaks the circle.
func TestBackfillReadsTheHistoryOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := func(model string, ctx int) string {
		b, _ := json.Marshal(map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"role": "assistant", "model": model,
				"usage": map[string]any{"input_tokens": ctx, "output_tokens": 10},
			},
		})
		return string(b)
	}
	// A session of the model HQ runs, which once held far more than the tier guess.
	if err := os.WriteFile(filepath.Join(dir, "old.jsonl"),
		[]byte(line("big-model", 998_881)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Before: the tier guess, which is the bug.
	if got := windowFor("claude", "big-model", 169_485, 0); got != 200_000 {
		t.Fatalf("precondition: got %d", got)
	}

	BackfillWindows()
	if got := windowFor("claude", "big-model", 169_485, 0); got != 1_000_000 {
		t.Errorf("after the backfill windowFor = %d, want 1,000,000", got)
	}

	// Once. A rescan on every tick would read the whole history all day.
	if err := os.WriteFile(filepath.Join(dir, "newer.jsonl"),
		[]byte(line("other-model", 900_000)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	BackfillWindows()
	if got := evidencedWindow("claude", "other-model"); got != 0 {
		t.Errorf("the backfill ran a second time (found %d for a log written after it)", got)
	}
}
