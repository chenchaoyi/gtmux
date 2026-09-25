package knowledge

import (
	"regexp"
	"strings"
)

// The matchers behind the ai-voice check, and the prose reader they run on. The RULES
// they enforce, with their tiers and their examples, live in plainlang.go: one table,
// read by the lint, by `gtmux knowledge style` and by the playbook's pointer.
//
// Why this is a lint and not a habit. On 2026-09-16 the whole base was rewritten by hand
// to read like a person — 485 of 505 entries touched, bold cut from 4987 places to 101,
// dashes from 2274 to 63, and five in-house coinages replaced with the plain words. None
// of that holds by itself: the next entry is written by an agent, and the base is echoed
// into every dispatch, so a slide back is not inert. What a check can hold is the
// MECHANICAL part of that pass, which is most of what the pass actually did.
//
// It reads PROSE only. Fenced blocks, inline code, indented blocks, table rows and
// quoted spans (「…」, “…”) are skipped: a command with `--flag`, a warning quoted as it
// was printed, a table of dashes, the commander's own words — none of these are this
// check's business, and flagging them is how a style check earns its way into being
// ignored. It was measured against a real base first (508 entries): reading everything it found
// 61 entries; reading prose only, 27. A check that fires on a fifth of what it reads is
// one nobody works down.

var (
	voiceFenceRe  = regexp.MustCompile("(?s)```.*?```")
	voiceInlineRe = regexp.MustCompile("`[^`\n]*`")
	voiceQuoteRe  = regexp.MustCompile(`「[^」\n]{0,400}」|“[^”\n]{0,400}”`)
	voiceDashRe   = regexp.MustCompile(`——|—|(?:^|\s)--(?:\s|$)`)
	voiceBoldRe   = regexp.MustCompile(`\*\*[^*\n]+\*\*`)
	voiceArrowRe  = regexp.MustCompile(`⇒`)
	// Chat residue: a greeting, a compliment or an offer that survived into the entry.
	voiceResidueRe = regexp.MustCompile(`(?i)希望(?:这)?(?:对你)?有帮助|好问题|当然可以|I hope this helps|Great question|You['’]re absolutely right`)
	// The staging contrast, both languages. Corroboration only — see the note above.
	voiceNotXRe = regexp.MustCompile(`(?i)不是[^。；\n]{1,40}[，,]\s*(?:而是|是)|\bnot (?:just|only|merely)\b[^.\n]{1,60}\bbut\b`)
)

// voiceJargon are in-house coinages with a plain word that says the same thing. These four
// are the ones the 2026-09-16 pass replaced; a base that never used them simply never
// trips this. The charter states the RULE (a term is explained, not banned, and a name
// only this machine uses never stands alone) and deliberately does not carry this list:
// the rule travels to every user, the list is one base's history.
//
// `敲门` is absent on purpose. The shipped charter uses it as the name of the wake knock,
// so an entry using it is following the charter, not drifting from it.
var voiceJargon = []struct{ word, say string }{
	{"船", "会话"},
	{"同族", "同类"},
	{"反验", "反向验证"},
	{"量具", "测量工具"},
}

// voiceProse strips what this check has no business reading.
func voiceProse(text string) string {
	s := voiceFenceRe.ReplaceAllString(text, " ")
	s = voiceInlineRe.ReplaceAllString(s, " ")
	s = voiceQuoteRe.ReplaceAllString(s, " ")
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "|") { // a table row
			continue
		}
		if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") { // an indented block
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

// voiceCheck is the ai-voice detail for a piece of text, or "" when it reads fine. It is
// the text-only entrance: the rules that need an entry's shape (its id, its title) find
// nothing here, which is what the callers passing raw text want.
func voiceCheck(text string) string {
	for _, f := range mechanicalFindings(entryProse{All: voiceProse(text)}) {
		if f.check == checkVoice {
			return f.detail
		}
	}
	return ""
}
