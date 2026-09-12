package knowledge

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The knowledge base's outputs, pinned byte for byte, so the extraction into its own
// package (hq-knowledge-engine, phase 1) is provably a pure move: every rendered file,
// the JSON index, every entry, and the CLI's own prints must come out identical before
// and after. The fixture is synthetic and fully pinned (fixed timestamps, fixed seqs) —
// nothing from a real ledger.
//
// Regenerate with: go test ./internal/... -run TestKnowledgeGolden -update

var updateGolden = flag.Bool("update", false, "rewrite the knowledge golden files")

const goldenNow = int64(1789000000) // 2026-09-10T…Z, fixed

func goldenDir(t *testing.T) string {
	t.Helper()
	// The goldens live beside the package the code is moving INTO, so they survive the move.
	wd, _ := os.Getwd()
	for d := wd; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return filepath.Join(d, "internal", "knowledge", "testdata", "golden")
		}
	}
	t.Fatal("go.mod not found")
	return ""
}

func goldenFixture(t *testing.T) {
	t.Helper()
	asHQ(t)
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	ops := []knowledgeOp{
		{V: 1, Op: knowledgeOpTopic, ID: "datasets", Title: "where the eval sets live", At: goldenNow - 9000, Seq: 100},
		{V: 1, Op: knowledgeOpAdd, ID: "pitfalls/wrangler-tls-resets", Topic: "pitfalls", Title: "wrangler TLS-resets from the office network",
			Body: "Retry once. Root cause is the corporate proxy.\n\n同族 [[best-practices/retry-idempotent-calls]].", At: goldenNow - 8000, Seq: 110, Seqs: []int64{101, 102}, Pane: "%21", Capture: "pitfalls/wrangler-tls"},
		{V: 1, Op: knowledgeOpAdd, ID: "best-practices/retry-idempotent-calls", Topic: "best-practices", Title: "retry only idempotent calls",
			Body: "A retry on a non-idempotent call is a duplicate side effect.", At: goldenNow - 7000, Seq: 120},
		{V: 1, Op: knowledgeOpAdd, ID: "workflows/release-flow", Topic: "workflows", Title: "release flow: tag, then wait for the app job",
			Body: "1. tag\n2. watch release.yml\n3. gtmux update", At: goldenNow - 6000, Seq: 130, Task: "t-9"},
		{V: 1, Op: knowledgeOpSupersede, ID: "workflows/release-flow-v2", Topic: "workflows", Title: "release flow: tag, wait, then install",
			Body: "1. tag\n2. watch release.yml\n3. gtmux update\n4. gtmux hq to re-seed", At: goldenNow - 5000, Seq: 140, Supersedes: "workflows/release-flow", Why: "the re-seed step was missing"},
		{V: 1, Op: knowledgeOpAdd, ID: "environment/office-dns", Topic: "environment", Title: "office DNS blocks the tunnel's final hop",
			Body: "Test on cellular.", At: goldenNow - 4000, Seq: 150},
		{V: 1, Op: knowledgeOpRetire, ID: "environment/office-dns", At: goldenNow - 3000, Seq: 160, Why: "the office moved"},
		{V: 1, Op: knowledgeOpAdd, ID: "datasets/eval-set-location", Topic: "datasets", Title: "the eval set lives under data/eval",
			Body: "Regenerate with make eval.", At: goldenNow - 2500, Seq: 165},
		{V: 1, Op: knowledgeOpAdd, ID: "corrections/verify-before-claiming-done", Topic: "corrections", Title: "verify before claiming done",
			Body: "\"Did you even run it\" — run what the change puts on screen.", At: goldenNow - 2000, Seq: 170, SeqRange: "150..170"},
		{V: 1, Op: knowledgeOpPromote, ID: "corrections/verify-before-claiming-done", At: goldenNow - 1500, Seq: 180, Why: "holds on every machine", Target: "gtmux playbook"},
		{V: 1, Op: knowledgeOpPromote, ID: "pitfalls/wrangler-tls-resets", At: goldenNow - 1200, Seq: 185, Why: "every office user hits it", Target: "docs/design/remote-access-tunnel.md"},
		{V: 1, Op: knowledgeOpLand, ID: "pitfalls/wrangler-tls-resets", At: goldenNow - 1000, Seq: 190, Ref: "PR #1"},
	}
	f, err := os.Create(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		b, _ := json.Marshal(op)
		f.Write(append(b, '\n'))
	}
	f.Close()
	for _, c := range []Candidate{
		{At: goldenNow - 900, Topic: "pitfalls", Key: "pitfalls/wrangler-tls", Lesson: "wrangler needs a retry (again)", Pane: "%30", Seq: 195},
		{At: goldenNow - 800, Topic: "corrections", Key: "corrections/mined-abc123def456", Lesson: "这个不对，重新做", Seq: 196,
			Source: "transcript", Context: "…moved the toggle and shipped it.", Session: "s1", Project: "demo"},
		{At: goldenNow - 700, Topic: "pitfalls", Key: "pitfalls/mined-bash-wrangler-command-not", Lesson: "bash: wrangler: command not found (×5, 3 sessions)", Seq: 197, Source: "transcript", Count: 5},
	} {
		if err := AppendCandidate(c); err != nil {
			t.Fatal(err)
		}
	}
}

func goldenStdout(t *testing.T, run func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	run()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestKnowledgeGolden(t *testing.T) {
	dir := goldenDir(t) // before the fixture chdirs into the temp HQ home
	goldenFixture(t)
	got := map[string]string{}

	live, custom, err := readKnowledgeState()
	if err != nil {
		t.Fatal(err)
	}
	if err := renderAllTopics(live, custom, goldenNow); err != nil {
		t.Fatal(err)
	}
	if err := renderPromotions(live); err != nil {
		t.Fatal(err)
	}
	root := Dir()
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		got["render/"+rel] = string(b)
		return nil
	})
	idx, err := KnowledgeIndexJSON(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	got["api/index.json"] = string(idx) + "\n"
	for _, op := range live {
		b, ok, err := KnowledgeEntryJSON(op.ID)
		if err != nil || !ok {
			t.Fatalf("entry %s: ok=%v err=%v", op.ID, ok, err)
		}
		got["api/entry-"+strings.ReplaceAll(op.ID, "/", "__")+".json"] = string(b) + "\n"
	}
	got["cli/list.json"] = goldenStdout(t, func() { CmdKnowledge([]string{"list", "--json"}) })
	got["cli/list.txt"] = goldenStdout(t, func() { CmdKnowledge([]string{"list"}) })
	got["cli/show.txt"] = goldenStdout(t, func() { CmdKnowledge([]string{"show", "workflows/release-flow-v2"}) })
	got["cli/capture-list.json"] = goldenStdout(t, func() { CmdCapture([]string{"--list", "--json"}, func(int64) string { return "" }) })

	if *updateGolden {
		os.RemoveAll(dir)
		for name, content := range got {
			p := filepath.Join(dir, name+".golden")
			os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("wrote %d golden files under %s", len(got), dir)
		return
	}
	var names []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".golden") {
			rel, _ := filepath.Rel(dir, p)
			names = append(names, strings.TrimSuffix(rel, ".golden"))
		}
		return nil
	})
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no golden files; run with -update first")
	}
	seen := map[string]bool{}
	for _, name := range names {
		seen[name] = true
		want, _ := os.ReadFile(filepath.Join(dir, name+".golden"))
		if g, ok := got[name]; !ok {
			t.Errorf("%s: no longer produced", name)
		} else if !bytes.Equal([]byte(g), want) {
			t.Errorf("%s: output changed\n--- want\n%s\n--- got\n%s", name, want, g)
		}
	}
	for name := range got {
		if !seen[name] {
			t.Errorf("%s: produced but no golden (run -update if intended)", name)
		}
	}
}
