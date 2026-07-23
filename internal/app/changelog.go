package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// What changed, in the user's terms (update-changelog).
//
// `gtmux update` used to end at "Installed. gtmux v0.39.0" — the one thing it never said
// was what you got. The generated commit list isn't an answer: it is addressed to whoever
// reads the diff, and a sha plus a PR number plus an author is noise to someone who only
// wants to know whether something they rely on moved.
//
// So a release carries a SECOND section, written for the user, in the TAG MESSAGE:
//
//	user:
//	- spawn --title now names the session
//	- restore returns you to the window you were on
//
// goreleaser copies the tag body into the release, and this file reads it back. Two rules
// make it trustworthy:
//
//   - It AGGREGATES ACROSS VERSIONS. Someone five releases behind is told about all five;
//     describing only the newest would be a lie of omission for exactly the user who most
//     needs the summary.
//   - It is SILENT when it has nothing. No release notes, no network — print nothing.
//     A "couldn't fetch the changelog" error after a successful install is noise about a
//     cosmetic step, and reads as though the update itself went wrong.

// userLinePrefix marks the section of a tag message written for users.
const userLinePrefix = "user:"

// changelogMax is how many lines `gtmux update` prints. The reader just ran an update and
// is on their way somewhere else; past a handful this stops being a summary. The rest is
// what `gtmux whatsnew` is for.
const changelogMax = 5

// release is the slice of the GitHub releases API we read.
type release struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
}

// VersionEntry is one version's user-facing notes.
type VersionEntry struct {
	Tag   string
	Lines []string
}

// fetchReleases returns recent releases, newest first ("" tags dropped).
func fetchReleases() []release {
	b := httpGetBytes("https://api.github.com/repos/chenchaoyi/gtmux/releases?per_page=30", 10*time.Second)
	if b == nil {
		return nil
	}
	var out []release
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// userLines extracts the `user:` block from a release body: the lines after the marker,
// up to the first blank line or the generated changelog heading. Bullet markers are
// stripped so the caller controls presentation.
func userLines(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var out []string
	in, fenced := false, false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// A release body routinely QUOTES the convention it documents. Without fence
		// tracking the parser found the example inside a code block and reported the
		// fence and a stray quote character as changes — nonsense presented to the user
		// as "what changed". A marker only counts outside a fence, and a fence ends a
		// block that was already open.
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
			if in {
				break
			}
			continue
		}
		if fenced {
			continue
		}
		if !in {
			if strings.EqualFold(t, userLinePrefix) || strings.HasPrefix(strings.ToLower(t), userLinePrefix+" ") {
				in = true
				// `user: one thing` on the marker line itself is that one thing.
				if rest := strings.TrimSpace(t[len(userLinePrefix):]); rest != "" {
					out = append(out, rest)
				}
			}
			continue
		}
		// The block ends at a blank line or at the generated section below it.
		if t == "" || strings.HasPrefix(t, "#") {
			break
		}
		out = append(out, strings.TrimSpace(strings.TrimLeft(t, "-*• \t")))
	}
	return out
}

// changesBetween returns the user-facing notes for every version in (from, to], newest
// first. `from` and `to` may carry a leading "v".
//
// A version with no `user:` block contributes nothing rather than falling back to commit
// subjects: a release whose author wrote no user-facing note is one where there was
// nothing for a user to know, and inventing a line from a commit subject would put
// developer vocabulary in front of them.
func changesBetween(rels []release, from, to string) []VersionEntry {
	var out []VersionEntry
	for _, r := range rels {
		if !inVersionRange(r.TagName, from, to) {
			continue
		}
		if lines := userLines(r.Body); len(lines) > 0 {
			out = append(out, VersionEntry{Tag: strings.TrimPrefix(r.TagName, "v"), Lines: lines})
		}
	}
	return out
}

// inVersionRange reports whether tag is in (from, to]. An unparseable bound admits
// everything up to `to`, so a missing "previous version" still yields the newest notes
// rather than nothing.
func inVersionRange(tag, from, to string) bool {
	t, ok := parseSemver(tag)
	if !ok {
		return false
	}
	if hi, ok := parseSemver(to); ok && compareSemver(t, hi) > 0 {
		return false
	}
	if lo, ok := parseSemver(from); ok && compareSemver(t, lo) <= 0 {
		return false
	}
	return true
}

// parseSemver reads "vX.Y.Z" / "X.Y.Z" into comparable parts. Trailing pre-release text
// is ignored for ordering — releases here are plain triples.
func parseSemver(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return v, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// flatten turns per-version entries into at most `max` lines, plus how many were left
// out. Newest first, so a truncated list keeps what changed most recently.
func flatten(entries []VersionEntry, max int) (lines []string, omitted int) {
	for _, e := range entries {
		for _, l := range e.Lines {
			if len(lines) < max {
				lines = append(lines, l)
			} else {
				omitted++
			}
		}
	}
	return lines, omitted
}

// cmdWhatsnew implements `gtmux whatsnew` — the full list layer 1 summarises.
//
// It defaults to "since the version you are running", which is the question someone
// actually has after an update told them there was more. `--since` widens it.
func cmdWhatsnew(args []string) int {
	since := strings.TrimPrefix(Version, "v")
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			i18n.Say("usage: gtmux whatsnew [--since vX.Y.Z] [--all]",
				"用法：gtmux whatsnew [--since vX.Y.Z] [--all]")
			i18n.Say("  What changed for YOU, per release — the full list `gtmux update` summarises.",
				"  每个版本对你而言的变化 —— `gtmux update` 只印摘要，这里是全部。")
			i18n.Say("  Defaults to everything newer than the version you're running.",
				"  默认显示比你当前版本更新的所有条目。")
			return 0
		case a == "--all":
			since = "0.0.0"
		case a == "--since":
			if i+1 >= len(args) {
				i18n.Sae("gtmux whatsnew: --since needs a version", "gtmux whatsnew: --since 需要一个版本号")
				return 2
			}
			i++
			since = strings.TrimPrefix(args[i], "v")
		case strings.HasPrefix(a, "--since="):
			since = strings.TrimPrefix(strings.TrimPrefix(a, "--since="), "v")
		default:
			i18n.Sae("gtmux whatsnew: unknown option '"+a+"'", "gtmux whatsnew: 未知选项 '"+a+"'")
			return 2
		}
	}

	rels := fetchReleases()
	if rels == nil {
		i18n.Sae("gtmux whatsnew: couldn't reach the release API (network?).",
			"gtmux whatsnew: 连不上发布接口（网络问题？）。")
		return 1
	}
	// An open upper bound: everything published, not just up to what's installed — the
	// point of asking is often to see whether it's worth updating.
	entries := changesBetween(rels, since, "")
	if len(entries) == 0 {
		i18n.Say("Nothing newer than "+since+".", "没有比 "+since+" 更新的内容。")
		return 0
	}
	for _, e := range entries {
		fmt.Println()
		i18n.Say(i18n.Bold+"  v"+e.Tag+i18n.Reset, i18n.Bold+"  v"+e.Tag+i18n.Reset)
		for _, l := range e.Lines {
			fmt.Println("    · " + l)
		}
	}
	fmt.Println()
	return 0
}
