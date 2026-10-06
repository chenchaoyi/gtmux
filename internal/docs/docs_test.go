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
	out, err := markedDocs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no document carries a rendered region — the marker moved and this guard stopped guarding")
	}
	return out
}

// docRoots are where documentation lives, searched to any depth. It used to be
// docs/*.md and the README pair only, so a marked region in docs/design/… or a
// translation beside it was checked by nothing (%12, 2026-10-06). openspec/changes is
// left out on purpose: a proposal quotes what it changes, and the archive is history.
var docRoots = []string{"docs", "api", filepath.Join("openspec", "specs")}

// markedDocs returns every .md file under root's documentation that carries a rendered
// region, sorted.
func markedDocs(root string) ([]string, error) {
	var out []string
	consider := func(p string) error {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), marker) {
			out = append(out, p)
		}
		return nil
	}
	for _, readme := range []string{"README.md", "README.zh.md"} {
		if err := consider(filepath.Join(root, readme)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, dir := range docRoots {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) && p == filepath.Join(root, dir) {
					return filepath.SkipDir
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".md") {
				return nil
			}
			return consider(p)
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// A marked region in a nested document, or in the translation beside it, is found.
func TestMarkedDocsAreFoundAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	marked := "x\n" + marker + " wake-lines -->\n"
	write("README.md", marked)
	write("docs/cli.md", marked)
	write("docs/design/sub/example.md", marked)
	write("docs/design/sub/example.zh.md", marked)
	write("api/contract.md", marked)
	write("openspec/specs/cap/spec.md", marked)
	write("docs/plain.md", "no marker here\n")
	write("openspec/changes/archive/old/proposal.md", marked) // history: not checked
	write("docs/notes.txt", marked)                           // not markdown
	got, err := markedDocs(root)
	if err != nil {
		t.Fatal(err)
	}
	var rel []string
	for _, p := range got {
		r, _ := filepath.Rel(root, p)
		rel = append(rel, filepath.ToSlash(r))
	}
	want := []string{"README.md", "api/contract.md", "docs/cli.md", "docs/design/sub/example.md", "docs/design/sub/example.zh.md", "openspec/specs/cap/spec.md"}
	if strings.Join(rel, " ") != strings.Join(want, " ") {
		t.Fatalf("found %q\nwant  %q", rel, want)
	}
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
