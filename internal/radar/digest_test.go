package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestSnip(t *testing.T) {
	if got := Snip("", 10); got != "" {
		t.Errorf("Snip empty = %q", got)
	}
	if got := Snip("a  b\n\tc", 10); got != "a b c" {
		t.Errorf("Snip whitespace = %q, want %q", got, "a b c")
	}
	long := strings.Repeat("字", 30)
	got := Snip(long, 10)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) != 11 {
		t.Errorf("Snip rune truncation = %q (runes %d)", got, len([]rune(got)))
	}
}

func TestJoinAsk(t *testing.T) {
	if got := joinAsk(nil); got != "" {
		t.Errorf("joinAsk(nil) = %q", got)
	}
	got := joinAsk([]prompt.Option{{N: 1, Label: "Yes"}, {N: 2, Label: "No, tell Claude what to do"}})
	want := "1.Yes · 2.No, tell Claude what to do"
	if got != want {
		t.Errorf("joinAsk = %q, want %q", got, want)
	}
}

func TestTurnDigest(t *testing.T) {
	if g, l := turnDigest(nil); g != "" || l != "" {
		t.Errorf("empty turns = (%q,%q)", g, l)
	}
	turns := []transcript.Turn{
		{Prompt: "old", Response: "old reply"},
		{Prompt: "fix the login bug", Response: "Found it.\nThe token was expired — patched and tests pass."},
	}
	g, l := turnDigest(turns)
	if g != "fix the login bug" {
		t.Errorf("goal = %q", g)
	}
	if !strings.Contains(l, "tests pass") {
		t.Errorf("last should carry the reply tail, got %q", l)
	}
	// A very long reply keeps its TAIL (the current end), marked with a leading …
	long := transcript.Turn{Prompt: "p", Response: strings.Repeat("x ", 500) + "THE END"}
	_, l = turnDigest([]transcript.Turn{long})
	if !strings.HasPrefix(l, "…") || !strings.HasSuffix(l, "THE END") {
		t.Errorf("long reply should tail-truncate, got %q…%q", l[:10], l[len(l)-10:])
	}
}

// roleForCwd marks ONLY the hq home; "" and other dirs stay unmarked.
func TestRoleForCwd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := roleForCwd(state.HQHome()); got != "supervisor" {
		t.Errorf("hq home role = %q, want supervisor", got)
	}
	if got := roleForCwd(""); got != "" {
		t.Errorf("empty cwd role = %q", got)
	}
	if got := roleForCwd(filepath.Join(os.Getenv("HOME"), "code")); got != "" {
		t.Errorf("other cwd role = %q", got)
	}
}

// senseOf grades a row's perception tier from facts the digest already holds —
// the agent-drivers contract addition (additive, omitempty).
func TestSenseOf(t *testing.T) {
	cases := []struct {
		hooked, content bool
		want            string
	}{
		{true, true, "driver"},
		{true, false, "partial"},
		{false, false, "screen"},
	}
	for _, c := range cases {
		if got := senseOf(c.hooked, c.content); got != c.want {
			t.Errorf("senseOf(%v,%v) = %q, want %q", c.hooked, c.content, got, c.want)
		}
	}
}

// The contract is additive: sense marshals as `sense`, is omitted when unset,
// and no pre-existing key moves — an old consumer parses the row unchanged.
func TestDigestRow_SenseIsAdditive(t *testing.T) {
	b, err := json.Marshal(DigestRow{Agent: "claude", Source: "tmux", Status: "idle", Sense: "driver"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"sense":"driver"`) {
		t.Fatalf("sense must marshal; got %s", b)
	}
	b, _ = json.Marshal(DigestRow{Agent: "claude", Source: "tmux", Status: "idle"})
	if strings.Contains(string(b), "sense") {
		t.Fatalf("an unset sense must be omitted; got %s", b)
	}
}
