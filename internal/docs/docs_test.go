package docs

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// -update rewrites the marked regions from the code instead of asserting them. This is
// `make docs-fix`; CI never passes it. The loop is: change a builder → CI goes red →
// `make docs-fix` → commit. Nobody hand-copies a rendered line, which is how the doc
// came to show a format the builder could no longer produce.
var update = flag.Bool("update", false, "rewrite the marked doc regions from the code")

// checked is every document carrying rendered regions, found by looking rather than by
// listing. The list it replaces held one entry, "docs/cli.md" — and docs/cli.zh.md
// carries the same two marked regions, checked by nothing and rewritten by nothing. A
// fabricated line pasted into the Chinese half passed both `go test ./...` and the
// design gate, and `make docs-fix` left it there.
//
// That is the failure this package was built to end, one language over: the package doc
// says "nobody hand-transcribes a line again", which was true only of the half somebody
// remembered to list. A hand-maintained list of the files a checker checks is the same
// shape of hole as a hand-maintained list of the things it checks.
func checkedDocs(t *testing.T) []string {
	t.Helper()
	var out []string
	roots, err := filepath.Glob(filepath.Join("..", "..", "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	roots = append(roots, filepath.Join("..", "..", "README.md"), filepath.Join("..", "..", "README.zh.md"))
	for _, p := range roots {
		b, err := os.ReadFile(p)
		if err != nil {
			continue // README twins are optional; a glob hit that vanished is not our business
		}
		if strings.Contains(string(b), marker) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		t.Fatal("no document carries a rendered region — the marker moved and this guard stopped guarding")
	}
	sort.Strings(out)
	return out
}

// The guard itself: every marked example in the docs is what the code really produces.
func TestDocExamples(t *testing.T) {
	marked := map[string]bool{} // every id some checked document marks
	for _, path := range checkedDocs(t) {
		doc := filepath.Base(path)
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
		regions, err := Regions(string(src))
		if err != nil {
			t.Fatalf("%s: %v", doc, err)
		}
		for _, r := range regions {
			marked[r.ID] = true
			produced, ok := Render(r.ID)
			if !ok {
				t.Errorf("%s: region %q — no registry entry; the doc marks an example "+
					"nothing renders\n  documented: %s", doc, r.ID, r.Body)
				continue
			}
			if r.Body != produced {
				t.Errorf("%s: region %q — the documented example is not what the code "+
					"produces\n  documented: %s\n  produced:   %s\n"+
					"  run `make docs-fix` to rewrite it from the code",
					doc, r.ID, r.Body, produced)
			}
		}
	}
	if *update {
		return // a rewrite pass records no `marked` set; the assert pass below is the guard
	}
	// A registered example nothing marks any more is drift too: the doc it guarded is
	// gone and nobody noticed the guard stopped guarding. Asked across every document at
	// once, because an id may legitimately live in one of them and not another.
	for id := range Examples {
		if !marked[id] {
			t.Errorf("registry entry %q is marked by no document — the example was "+
				"removed or renamed\n  produced: %s", id, mustRender(id))
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

// fullSample is a synthetic doc carrying one stale region per REGISTERED id, built from the
// registry rather than written out. The tests that call Check on a whole document need every
// id present (a registered id with no region is itself a finding), and a hand-written
// fixture would make registering a second example fail these tests for no reason — which is
// exactly what happened when `unread-line` was added.
func fullSample(t *testing.T) string {
	t.Helper()
	ids := make([]string, 0, len(Examples))
	for id := range Examples {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	b.WriteString("intro\n")
	for _, id := range ids {
		b.WriteString(marker + id + " -->\n```\nLINE ONE\nLINE TWO\n```\n")
	}
	b.WriteString("outro\n")
	return b.String()
}

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
	var got *Mismatch
	for i, m := range bad {
		if m.ID == "wake-lines" {
			got = &bad[i]
		}
	}
	if got == nil {
		t.Fatalf("a stale region must be reported; got %+v", bad)
	}
	if !strings.Contains(got.Produced, "gtmux·waiting·permission") {
		t.Errorf("the report must show what the code DOES produce; got %q", got.Produced)
	}
	if !strings.Contains(got.Documented, "[gtmux]") {
		t.Errorf("…and what the doc claims; got %q", got.Documented)
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
	out, err := Rewrite(fullSample(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "intro\n") || !strings.HasSuffix(out, "outro\n") {
		t.Fatalf("prose outside the region must be untouched:\n%s", out)
	}
	if strings.Contains(out, "LINE ONE") {
		t.Error("the stale body should have been replaced")
	}
	if !strings.Contains(out, "gtmux·done") {
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
