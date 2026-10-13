package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWhatsnewJSONBoundsBothLanguagesAndNeverUsesCommits(t *testing.T) {
	rels := []release{
		{TagName: "v1.0.2", Body: "user-zh:\n- 只有中文\n\n## Changelog\n* developer subject"},
		{TagName: "v1.0.3", Body: "user:\n- English\n\nuser-zh:\n- 中文"},
		{TagName: "v1.0.4", Body: "user:\n- Not installed"},
		{TagName: "v1.0.5", Body: "user:\n- Draft", Draft: true},
		{TagName: "v1.0.6-rc.1", Body: "user:\n- Preview", Prerelease: true},
		{TagName: "v1.0.1", Body: "## Changelog\n* Internal refactor"},
	}
	rc := -1
	output := captureStdout(t, func() {
		rc = runWhatsnew([]string{"--since", "v1.0.1", "--until=v1.0.3", "--json"}, func() []release { return rels })
	})
	if rc != 0 {
		t.Fatalf("exit = %d", rc)
	}
	var r releaseNotesJSON
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		t.Fatal(err)
	}
	if r.Schema != 1 || r.Limit != 30 || r.Since != "1.0.1" || r.Until != "1.0.3" || r.PublishedThrough != "1.0.4" || r.OldestAvailable != "1.0.1" {
		t.Fatalf("receipt=%+v", r)
	}
	if len(r.Entries) != 2 || r.Entries[0].Version != "1.0.3" || r.Entries[1].Version != "1.0.2" {
		t.Fatalf("entries=%+v", r.Entries)
	}
	if len(r.Entries[0].EN) != 1 || r.Entries[0].EN[0] != "English" || r.Entries[0].ZH[0] != "中文" || len(r.Entries[1].EN) != 0 || r.Entries[1].ZH[0] != "只有中文" {
		t.Fatalf("languages=%+v", r.Entries)
	}
	for _, bad := range []string{"developer subject", "Internal refactor", "Not installed", "Draft", "Preview", "null"} {
		if strings.Contains(output, bad) {
			t.Fatalf("unexpected JSON content %q: %s", bad, output)
		}
	}
}

func TestWhatsnewInvalidUpperBoundAndFailureDoNotLookLikeEmptySuccess(t *testing.T) {
	for _, args := range [][]string{{"--until"}, {"--until="}, {"--until", "bad"}} {
		called := false
		rc := runWhatsnew(args, func() []release { called = true; return []release{} })
		if rc != 2 || called {
			t.Fatalf("%v: rc=%d fetched=%v", args, rc, called)
		}
	}
	rc := -1
	output := captureStdout(t, func() {
		rc = runWhatsnew([]string{"--all", "--until", "1.0.3", "--json"}, func() []release { return nil })
	})
	if rc != 1 || output != "" {
		t.Fatalf("network failure: rc=%d output=%q", rc, output)
	}
	output = captureStdout(t, func() {
		rc = runWhatsnew([]string{"--since", "1.0.3", "--until", "1.0.3", "--json"}, func() []release { return []release{{TagName: "v1.0.3", Body: notes}} })
	})
	if rc != 0 || !strings.Contains(output, `"entries":[]`) {
		t.Fatalf("empty range: rc=%d output=%q", rc, output)
	}
}

func TestWhatsnewTextKeepsOpenUpperBoundUnlessRequested(t *testing.T) {
	rels := []release{{TagName: "v1.0.4", Body: "user:\n- Future feature"}, {TagName: "v1.0.3", Body: "user:\n- Installed feature"}}
	output := captureStdout(t, func() {
		if rc := runWhatsnew([]string{"--since", "1.0.2"}, func() []release { return rels }); rc != 0 {
			t.Fatal(rc)
		}
	})
	if !strings.Contains(output, "Future feature") || !strings.Contains(output, "Installed feature") {
		t.Fatal(output)
	}
	output = captureStdout(t, func() {
		if rc := runWhatsnew([]string{"--all", "--until", "1.0.3"}, func() []release { return rels }); rc != 0 {
			t.Fatal(rc)
		}
	})
	if strings.Contains(output, "Future feature") || !strings.Contains(output, "Installed feature") {
		t.Fatal(output)
	}
}
