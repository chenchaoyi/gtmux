package app

import (
	"reflect"
	"testing"
)

// `gtmux update` ended at "Installed. gtmux v0.39.0" — never saying what you got. These
// pin the summary that replaced that silence (update-changelog).

const notes = `user:
- spawn --title now names the session
- restore returns you to the window you were on

## Changelog
* aaef1de: fix(spawn): let --title name the session (#550) (@chenchaoyi)`

func TestUserLinesReadsTheBlockAndStopsAtTheGeneratedList(t *testing.T) {
	got := userLines(notes)
	want := []string{"spawn --title now names the session", "restore returns you to the window you were on"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v; want the two user lines, bullets stripped and no commit noise", got)
	}
	// The generated section must never leak in — a sha and an @author is exactly the
	// vocabulary this exists to avoid.
	for _, l := range got {
		for _, bad := range []string{"aaef1de", "@chenchaoyi", "#550", "fix("} {
			if contains(l, bad) {
				t.Errorf("line %q leaked developer vocabulary (%q)", l, bad)
			}
		}
	}
}

func TestUserLinesIsEmptyWhenTheReleaseHasNoBlock(t *testing.T) {
	for _, body := range []string{"", "## Changelog\n* abc123: chore: bump deps"} {
		if got := userLines(body); len(got) != 0 {
			t.Errorf("body %q → %#v; want nothing (a release with no user block says nothing)", body, got)
		}
	}
}

// The point of the feature: someone several versions behind is told about ALL of them.
func TestChangesAggregateAcrossEveryVersionCrossed(t *testing.T) {
	rels := []release{
		{TagName: "v0.39.0", Body: "user:\n- newest thing"},
		{TagName: "v0.38.1", Body: "user:\n- middle thing"},
		{TagName: "v0.38.0", Body: "user:\n- older thing"},
		{TagName: "v0.37.1", Body: "user:\n- already had this"},
	}
	got := changesBetween(rels, "0.37.1", "0.39.0")
	if len(got) != 3 {
		t.Fatalf("got %d versions; want the 3 crossed (0.38.0, 0.38.1, 0.39.0)", len(got))
	}
	if got[0].Tag != "0.39.0" {
		t.Errorf("first entry = %s; want newest first", got[0].Tag)
	}
	// The version you were ALREADY on is not news.
	for _, e := range got {
		if e.Tag == "0.37.1" {
			t.Error("included the version the user already had")
		}
	}
}

// A version ABOVE the one just installed must not be announced as if you got it.
func TestChangesExcludeVersionsNewerThanInstalled(t *testing.T) {
	rels := []release{
		{TagName: "v0.40.0", Body: "user:\n- not installed yet"},
		{TagName: "v0.39.0", Body: "user:\n- what you got"},
	}
	got := changesBetween(rels, "0.38.0", "0.39.0")
	if len(got) != 1 || got[0].Tag != "0.39.0" {
		t.Fatalf("got %#v; want only 0.39.0", got)
	}
}

// whatsnew asks with an OPEN upper bound — "is it worth updating" needs the unreleased-
// to-me versions included.
func TestAnOpenUpperBoundIncludesEverythingNewer(t *testing.T) {
	rels := []release{
		{TagName: "v0.40.0", Body: "user:\n- newer"},
		{TagName: "v0.39.0", Body: "user:\n- current"},
	}
	if got := changesBetween(rels, "0.39.0", ""); len(got) != 1 || got[0].Tag != "0.40.0" {
		t.Fatalf("got %#v; want 0.40.0 only", got)
	}
}

func TestFlattenCapsAndCountsTheRemainder(t *testing.T) {
	entries := []VersionEntry{
		{Tag: "0.39.0", Lines: []string{"a", "b", "c"}},
		{Tag: "0.38.0", Lines: []string{"d", "e", "f"}},
	}
	lines, omitted := flatten(entries, 4)
	if len(lines) != 4 || omitted != 2 {
		t.Fatalf("got %d lines / %d omitted; want 4 / 2", len(lines), omitted)
	}
	if lines[0] != "a" {
		t.Errorf("first line = %q; want the newest version's first line", lines[0])
	}
}

func TestSemverOrdering(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"v0.39.0", "v0.38.1", 1},
		{"0.38.1", "0.39.0", -1},
		{"v1.0.0", "1.0.0", 0},
		{"v0.10.0", "v0.9.0", 1}, // numeric, not lexical
	} {
		x, _ := parseSemver(c.a)
		y, _ := parseSemver(c.b)
		if got := compareSemver(x, y); got != c.want {
			t.Errorf("compare(%s,%s) = %d; want %d", c.a, c.b, got, c.want)
		}
	}
	if _, ok := parseSemver("not-a-version"); ok {
		t.Error("junk parsed as a version")
	}
}
