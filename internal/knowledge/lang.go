package knowledge

import (
	"strings"
	"unicode"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The knowledge base speaks two languages (change kb-bilingual). An entry records the
// language it was WRITTEN in and may carry the other language as an alternate half —
// written by HQ, never machine-translated (gtmux calls no model). Readers pick their
// language with one rule, `pick`: the source half when it matches, else the alternate
// when it matches, else the source with a tag saying which language that is.
//
// Records older than this change say nothing about their language; the fold infers it
// from the text (CJK ratio) and marks the record `langAssumed`, the same read-time
// migration the kinds got — the ledger is never rewritten.

// Langs is the product's two languages, in the order the vocabulary lists them.
var Langs = []string{"zh", "en"}

func validLang(l string) bool { return contains(Langs, l) }

// knowledgeAlt is the other language's half of an entry.
type altHalf struct {
	Lang  string `json:"lang"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// inferLang guesses the language an entry was written in. The title decides when it
// can: it is short and in the author's language, so any Han character makes it Chinese
// and Latin letters alone make it English. Only a title with no letters at all (a bare
// pane id, say) hands the decision to the body's CJK ratio, where identifiers and code
// do not vote — a Chinese lesson full of paths still reads as Chinese.
func inferLang(title, body string) string {
	if strings.ContainsFunc(title, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
		return "zh"
	}
	if strings.ContainsFunc(title, func(r rune) bool { return r < 128 && unicode.IsLetter(r) }) {
		return "en"
	}
	var cjk, latin int
	for _, r := range body {
		switch {
		case unicode.Is(unicode.Han, r):
			cjk++
		case r < 128 && unicode.IsLetter(r):
			latin++
		}
	}
	if cjk > 0 && cjk*5 >= latin {
		return "zh"
	}
	return "en"
}

// readerLang is the language the CLI's reader has (GTMUX_LANG through i18n), with an
// explicit `--lang` overriding it.
func readerLang(override string) string {
	if validLang(override) {
		return override
	}
	if l := i18n.Lang(); validLang(l) {
		return l
	}
	return "en"
}

// pick resolves an entry for a reader: the source half when its language matches, the
// alternate when that matches, else the source. `tag` is "" when the reader got their
// language and the source language otherwise, for the reader to see what they are
// reading. The three-step rule the menu bar and the phone apply too.
func pick(op knowledgeOp, lang string) (title, body, tag string) {
	if op.Lang == lang || !validLang(lang) {
		return op.Title, op.Body, ""
	}
	if op.Alt != nil && op.Alt.Lang == lang && op.Alt.Title != "" {
		return op.Alt.Title, op.Alt.Body, ""
	}
	return op.Title, op.Body, op.Lang
}

// machineLang is the language this machine's base is written in: the majority language
// of its live entries. Deliberately NOT the process's GTMUX_LANG — the daily sync runs
// under launchd with no language set, and a block that flipped between English and
// Chinese depending on who last rendered it would churn every agent's instruction file.
// A tie, or an empty base, reads as English.
func machineLang(live []knowledgeOp) string {
	var zh, en int
	for _, op := range live {
		switch op.Lang {
		case "zh":
			zh++
		case "en":
			en++
		}
	}
	if zh > en {
		return "zh"
	}
	return "en"
}

// briefLang is the language a promotion brief takes: English for `everyone` (the target
// is a public, English repository), the entry's own for the audiences on this machine.
func briefLang(op knowledgeOp) string {
	if op.Audience == AudienceEveryone {
		return "en"
	}
	return op.Lang
}

// altText is the alternate half as one string, for search: a duplicate written in the
// other language is still a duplicate.
func altText(op knowledgeOp) string {
	if op.Alt == nil {
		return ""
	}
	return strings.TrimSpace(op.Alt.Title + " " + op.Alt.Body)
}
