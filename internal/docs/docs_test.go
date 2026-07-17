package docs

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -update rewrites the marked regions from the code instead of asserting them. This is
// `make docs-fix`; CI never passes it. The loop is: change a builder → CI goes red →
// `make docs-fix` → commit. Nobody hand-copies a rendered line, which is how the doc
// came to show a format the builder could no longer produce.
var update = flag.Bool("update", false, "rewrite the marked doc regions from the code")

// checked lists the documents carrying rendered regions, relative to the repo root.
var checked = []string{"docs/cli.md"}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join("..", "..", rel)
}

// The guard itself: every marked example in the docs is what the code really produces.
func TestDocExamples(t *testing.T) {
	for _, doc := range checked {
		path := repoPath(t, doc)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", doc, err)
		}
		if *update {
			out, err := Rewrite(string(src))
			if err != nil {
				t.Fatalf("%s: %v", doc, err)
			}
			if out != string(src) {
				if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("%s: regions rewritten", doc)
			}
			continue
		}
		bad, err := Check(string(src))
		if err != nil {
			t.Fatalf("%s: %v", doc, err)
		}
		for _, m := range bad {
			t.Errorf("%s: region %q — %s\n  documented: %s\n  produced:   %s\n"+
				"  run `make docs-fix` to rewrite it from the code",
				doc, m.ID, m.Reason, m.Documented, m.Produced)
		}
	}
}

// ── the parser + the two failure directions ──────────────────────────────────

const sample = "intro\n" +
	"<!-- gtmux:rendered wake-lines -->\n" +
	"```\n" +
	"LINE ONE\n" +
	"LINE TWO\n" +
	"```\n" +
	"outro\n"

func TestRegions_ParsesTheFenceBody(t *testing.T) {
	got, err := Regions(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "wake-lines" || got[0].Body != "LINE ONE\nLINE TWO" {
		t.Fatalf("parsed = %+v", got)
	}
}

// An unmarked example is deliberately unchecked — regions are OPT-IN, the same escape
// the command-registry check gives with HIDDEN.
func TestRegions_IgnoresUnmarkedFences(t *testing.T) {
	got, err := Regions("```\nnot marked\n```\n")
	if err != nil || len(got) != 0 {
		t.Fatalf("an unmarked fence must be left alone; got %+v, %v", got, err)
	}
}

// A marker that guards nothing is an error, not a skip: someone meant to check an
// example, and silence would mean it never was.
func TestRegions_MarkerWithoutAFenceFails(t *testing.T) {
	if _, err := Regions("<!-- gtmux:rendered wake-lines -->\njust prose\n"); err == nil {
		t.Fatal("a marker not followed by a fence must fail loudly")
	}
	if _, err := Regions("<!-- gtmux:rendered wake-lines -->\n```\nunterminated\n"); err == nil {
		t.Fatal("an unterminated fence must fail loudly")
	}
}

// The check that would have caught #478: the builder's output moved, the doc didn't.
func TestCheck_CatchesAStaleExample(t *testing.T) {
	stale := strings.Replace(sample, "LINE ONE\nLINE TWO", "[gtmux] waiting·permission api:0.0 (%7) — old", 1)
	bad, err := Check(stale)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 1 || bad[0].ID != "wake-lines" {
		t.Fatalf("a stale region must be reported; got %+v", bad)
	}
	if !strings.Contains(bad[0].Produced, "» gtmux·waiting·permission") {
		t.Errorf("the report must show what the code DOES produce; got %q", bad[0].Produced)
	}
	if !strings.Contains(bad[0].Documented, "[gtmux]") {
		t.Errorf("…and what the doc claims; got %q", bad[0].Documented)
	}
}

// A registry entry whose region was deleted stops guarding anything — silently, unless
// we say so.
func TestCheck_CatchesADeadRegistryEntry(t *testing.T) {
	bad, err := Check("no regions here\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != len(Examples) {
		t.Fatalf("every registered id with no region must be reported; got %+v", bad)
	}
	if !strings.Contains(bad[0].Reason, "no region marks it") {
		t.Errorf("reason = %q", bad[0].Reason)
	}
}

// A doc marking an id nothing renders is the mirror image, and equally silent.
func TestCheck_CatchesAnUnregisteredRegion(t *testing.T) {
	src := strings.Replace(sample, "wake-lines", "never-registered", 1)
	bad, err := Check(src)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range bad {
		if m.ID == "never-registered" && strings.Contains(m.Reason, "no registry entry") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an unregistered region must be reported; got %+v", bad)
	}
}

// Rewrite touches the region and NOTHING else, and is idempotent.
func TestRewrite_OnlyTheRegion(t *testing.T) {
	out, err := Rewrite(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "intro\n") || !strings.HasSuffix(out, "outro\n") {
		t.Fatalf("prose outside the region must be untouched:\n%s", out)
	}
	if strings.Contains(out, "LINE ONE") {
		t.Error("the stale body should have been replaced")
	}
	if !strings.Contains(out, "» gtmux·done") {
		t.Errorf("the region should now hold the real rendering:\n%s", out)
	}
	again, err := Rewrite(out)
	if err != nil || again != out {
		t.Error("rewrite must be idempotent")
	}
	if bad, _ := Check(out); len(bad) != 0 {
		t.Errorf("a rewritten doc must pass the check; got %+v", bad)
	}
}
