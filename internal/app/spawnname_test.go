package app

import "testing"

// `gtmux spawn --title` states what a session is FOR, and it was reaching only the WINDOW
// name — the session itself kept a name derived from the goal, on every path. That is
// where live sessions called `你是一次性-worker(不是-HQ,不要` and `12` came from.

func TestSpawnSessionNamePrefersTheTitle(t *testing.T) {
	// The reported case: an explicit title must win over both branch and goal.
	got := spawnSessionName("restore contract", "feat/restore-contract", "帮我把 restore 的回归锁建起来，先跑红再修绿")
	if got != "restore-contract" {
		t.Errorf("name = %q; want the TITLE to name the session", got)
	}
	// Without a title, the branch; without either, the goal — unchanged behaviour.
	if got := spawnSessionName("", "feat/menubar-width", "whatever"); got != "feat-menubar-width" {
		t.Errorf("branch fallback = %q", got)
	}
	if got := spawnSessionName("", "", "fix the parser"); got != "fix-the-parser" {
		t.Errorf("goal fallback = %q", got)
	}
}

// The live ugly name, in its own terms: punctuation that made it noise is gone.
func TestSpawnSessionNameStripsPunctuationThatMadeNoise(t *testing.T) {
	got := spawnSessionName("", "", "你是一次性-worker(不是-HQ,不要")
	for _, bad := range []string{"(", ")", ",", "!", "--"} {
		if contains(got, bad) {
			t.Errorf("name %q still carries %q", got, bad)
		}
	}
	if got == "" {
		t.Error("sanitizing must not empty the name")
	}
}

// The old truncation was `name[:40]` — a BYTE slice, which cuts multi-byte scripts
// mid-character. Two Chinese words already exceed 40 bytes.
func TestSpawnSessionNameTruncatesByRuneNotByte(t *testing.T) {
	long := "这是一个非常长的中文目标描述用来验证截断不会把字符切坏掉真的很长"
	got := spawnSessionName("", "", long)
	if len([]rune(got)) > spawnNameMaxRunes {
		t.Errorf("name has %d runes; want ≤ %d", len([]rune(got)), spawnNameMaxRunes)
	}
	// Valid UTF-8 with no replacement character — the mojibake check.
	for _, r := range got {
		if r == '�' {
			t.Fatalf("name %q was cut mid-character", got)
		}
	}
}

// A collision used to fall through to tmux's own numeric auto-naming — the session
// called `12`. It must adjust the requested name instead of discarding it.
func TestCollidingNameIsSuffixedNotReplacedByANumber(t *testing.T) {
	live := map[string]bool{"review": true, "review-2": true}
	got := uniqueSessionName("review", func(n string) bool { return live[n] })
	if got != "review-3" {
		t.Errorf("name = %q; want review-3 (keep the requested name, adjust it)", got)
	}
	for _, r := range got {
		if r >= '0' && r <= '9' {
			continue
		}
		goto ok
	}
	t.Errorf("name %q is all digits — that is the bug (tmux's auto-numbering)", got)
ok:
	if got := uniqueSessionName("free", func(string) bool { return false }); got != "free" {
		t.Errorf("uncontested name = %q; want it unchanged", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A FAILED dispatch still leaves a live session behind, so the report has to name it. The
// failure branches printed the bare pane id — the one identifier you can't act on: you
// can't jump to it and it says nothing about which session or what the window is for.
func TestTheStandardHandleSurvivesAFailedDispatch(t *testing.T) {
	h := spawnHandle("gtmux:3.0", "%37", "restore-contract")
	for _, want := range []string{"gtmux:3.0", "%37", "restore-contract"} {
		if !contains(h, want) {
			t.Errorf("handle %q is missing %q", h, want)
		}
	}
	// Degrades rather than printing empty punctuation when a piece is unknown.
	if got := spawnHandle("", "%37", ""); got == "" || contains(got, "·") {
		t.Errorf("handle with only a pane = %q; want just the pane, no dangling separator", got)
	}
}

// ④ The reported path, asserted structurally: naming must be DECOUPLED from delivery.
//
// The report said --title stopped working "on the delivery-failure path". It never worked
// on ANY path — title wasn't a naming input at all. But the decoupling still has to be
// pinned, or a future change could make the claim true: the name is computed from its
// inputs alone and applied when the session is CREATED, before a byte of the goal is
// delivered, so a composer timeout minutes later cannot rename anything.
func TestSessionNameIsIndependentOfDelivery(t *testing.T) {
	const title, goal = "restore contract", "帮我把 restore 的回归锁建起来(先跑红,再修绿)"

	first := spawnSessionName(title, "", goal)
	for i := 0; i < 3; i++ {
		if got := spawnSessionName(title, "", goal); got != first {
			t.Fatalf("name changed between calls (%q → %q) — it must depend on nothing but its inputs", first, got)
		}
	}
	if first != "restore-contract" {
		t.Errorf("name = %q; want the title, not the goal head", first)
	}
	for _, frag := range []string{"回归锁", "先跑红", "("} {
		if contains(first, frag) {
			t.Errorf("name %q leaked the goal (%q) despite an explicit --title", first, frag)
		}
	}
}
