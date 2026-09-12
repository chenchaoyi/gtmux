// Knowledge RENDERS (hq-knowledge-ledger): each topic markdown file is a
// deterministic render of that topic's live ledger entries — gtmux-owned, marked,
// idempotent, and therefore DRIFT-DETECTABLE: a hand edit is caught by comparing
// the file to its render, not silently absorbed and not silently overwritten.
//
// Migration is incremental: the first mutation touching a topic moves its
// pre-ledger hand-written file VERBATIM to legacy/<topic>.md (a file still equal
// to its seeded placeholder is simply replaced), and the dispatch-time knowledge
// echo consults BOTH, so no lesson loses reach while HQ migrates by use.
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// knowledgeRenderMarker heads every rendered topic file. The version is the
// RENDER format's, independent of the ledger schema.
const knowledgeRenderMarker = "<!-- gtmux-hq-knowledge v1 · rendered from .ledger.jsonl — edit via `gtmux knowledge`, never by hand -->"

// knowledgeInlineBodyMax is the longest body flattened into the entry's bullet
// line; anything longer (or multi-line) indents beneath it. Bullet lines are what
// the dispatch-time echo greps, so short bodies staying on the bullet keeps them
// consultable.
const knowledgeInlineBodyMax = 300

// knowledgeLegacyDir holds pre-ledger hand-written topic files, moved verbatim.
func knowledgeLegacyDir() string { return filepath.Join(Dir(), "legacy") }

// knowledgePromotionsDir is the export OUTBOX: one brief per pending promotion,
// written on promote, removed on land, swept on every render pass.
func knowledgePromotionsDir() string { return filepath.Join(Dir(), "promotions") }

// knowledgePromotionMarker heads every brief.
const knowledgePromotionMarker = "<!-- gtmux-hq-promotion v1 · rendered from .ledger.jsonl — close the loop with `gtmux knowledge land <id> --ref <pr/spec>` -->"

// promotionBriefPath names a pending promotion's brief (the id's slash becomes a
// dash so the file sits flat in the outbox).
func promotionBriefPath(op knowledgeOp) string {
	return filepath.Join(knowledgePromotionsDir(), strings.ReplaceAll(op.ID, "/", "-")+".md")
}

// renderPromotionBrief is the carryable evidence package: the lesson, the
// promotion case, the suggested landing spot, the entry's provenance, and the
// closing instruction. Deterministic (UTC dates), like every other render.
func renderPromotionBrief(op knowledgeOp) string {
	var b strings.Builder
	b.WriteString(renderPromotionCore(op))
	renderPromotionExit(&b, op)
	return b.String()
}

// renderPromotionCore is the brief without its exit: what an issue body or a pasted
// block carries. IssueURL uses it — the exit for `everyone` IS the issue, so a body that
// contained the exit would contain its own URL.
func renderPromotionCore(op knowledgeOp) string {
	var b strings.Builder
	b.WriteString(knowledgePromotionMarker + "\n")
	b.WriteString("# promotion: " + op.Title + "\n\n")
	b.WriteString("- id: `" + op.ID + "` · promoted " + stampDate(op.PromotedAt) + "\n")
	switch {
	case op.Audience != "":
		aud := op.Audience
		if op.AudienceRepo != "" {
			aud += ":" + op.AudienceRepo
		}
		b.WriteString("- for: " + aud + " · " + audienceWord(op.Audience) + "\n")
	case op.PromoteTarget != "":
		b.WriteString("- suggested landing: " + op.PromoteTarget + "\n")
	}
	b.WriteString("- why charter-level: " + op.PromoteWhy + "\n\n")
	if body := strings.TrimSpace(op.Body); body != "" {
		b.WriteString(body + "\n\n")
	}
	b.WriteString("provenance: " + provenanceFooter(op) + "\n\n")
	return b.String()
}

// renderPromotionExit appends the closing instruction for the audience.
func renderPromotionExit(b *strings.Builder, op knowledgeOp) {
	// The closing instruction is the USER'S destination, never a hardcoded one
	// (hq-promote-anywhere): a promotion that named its target closes with it; one
	// that did not gets the carrier options — a brew-installed user has no gtmux
	// checkout, and their rules live in their own carriers.
	switch op.Audience {
	case AudienceHQ:
		b.WriteString("Exit: gtmux writes it into LOCAL.md and closes the loop:\n\n")
		b.WriteString("    gtmux knowledge land " + op.ID + "\n")
	case AudienceMachine:
		b.WriteString("Exit: gtmux renders " + MachinePath() + " and refreshes every agent's knowledge block, then closes:\n\n")
		b.WriteString("    gtmux knowledge land " + op.ID + "\n")
	case AudienceRepo:
		b.WriteString("Exit: gtmux writes it into " + RepoCarrierPath(op.AudienceRepo) + " (not committed — that is yours), then closes:\n\n")
		b.WriteString("    gtmux knowledge land " + op.ID + "\n")
	case AudienceEveryone:
		b.WriteString("Exit: this is feedback to gtmux. Open the prefilled issue, or paste this brief into one:\n\n")
		b.WriteString("    " + IssueURL(op) + "\n\n")
		b.WriteString("then close with the issue's URL:\n\n")
		b.WriteString("    gtmux knowledge land " + op.ID + " --ref \"<issue url>\"\n")
	default:
		if op.PromoteTarget != "" {
			b.WriteString("Land it at: " + op.PromoteTarget + " — then close:\n\n")
		} else {
			b.WriteString("No audience was chosen. `gtmux knowledge withdraw " + op.ID + " --why …` and promote again with " +
				"`--for <hq|machine|repo:<path>|everyone>`, or land it yourself and say where:\n\n")
		}
		b.WriteString("    gtmux knowledge land " + op.ID + " --ref \"<pr / issue / runbook>\"\n")
	}

}

// renderPromotions writes one brief per PENDING promotion and sweeps everything
// else out of the outbox — a landed or superseded promotion's brief must not
// linger as a stale hand-off. The sweep only touches files this renderer names
// (.md), so a stray user file is left alone.
func renderPromotions(live []knowledgeOp) error {
	pending, _ := pendingPromotions(live)
	if err := os.MkdirAll(knowledgePromotionsDir(), 0o755); err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, op := range pending {
		path := promotionBriefPath(op)
		expected[filepath.Base(path)] = true
		if err := os.WriteFile(path, []byte(renderPromotionBrief(op)), 0o644); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(knowledgePromotionsDir())
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || expected[e.Name()] {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(knowledgePromotionsDir(), e.Name())); err == nil &&
			strings.HasPrefix(string(b), knowledgePromotionMarker) {
			_ = os.Remove(filepath.Join(knowledgePromotionsDir(), e.Name()))
		}
	}
	return nil
}

func topicPath(topic string) string { return filepath.Join(Dir(), topic+".md") }

// renderTopic renders one topic's live entries (pure). desc is a declared custom topic's
// one-line description ("" for built-ins).
func renderTopic(topic, desc string, live []knowledgeOp) string {
	var b strings.Builder
	b.WriteString(knowledgeRenderMarker + "\n")
	b.WriteString("# " + topic + "\n\n")
	if desc != "" {
		b.WriteString("> " + desc + "\n\n")
	}
	if _, err := os.Stat(filepath.Join(knowledgeLegacyDir(), topic+".md")); err == nil {
		b.WriteString("> Pre-ledger hand-written entries: [legacy/" + topic + ".md](legacy/" +
			topic + ".md) — migrate the ones you touch.\n\n")
	}
	var hypotheses []knowledgeOp
	for _, op := range live {
		if op.Topic != topic {
			continue
		}
		if op.Status == StatusHypothesis {
			hypotheses = append(hypotheses, op)
			continue
		}
		renderEntry(&b, op)
	}
	if len(hypotheses) > 0 {
		b.WriteString("\n## unverified · 待验证\n\n")
		for _, op := range hypotheses {
			renderEntry(&b, op)
		}
	}
	return b.String()
}

// renderEntry writes one entry in the topic-file form.
func renderEntry(b *strings.Builder, op knowledgeOp) {
	{
		body := strings.TrimSpace(op.Body)
		if body != "" && !strings.Contains(body, "\n") && len(body) <= knowledgeInlineBodyMax {
			b.WriteString("- **" + op.Title + "** — " + body + "\n")
		} else {
			b.WriteString("- **" + op.Title + "**\n")
			for _, line := range strings.Split(body, "\n") {
				if strings.TrimSpace(line) == "" {
					b.WriteString("\n")
					continue
				}
				b.WriteString("    " + line + "\n")
			}
		}
		b.WriteString("  · " + provenanceFooter(op) + "\n")
	}
}

// provenanceFooter is the one-line evidence trail under each entry.
func provenanceFooter(op knowledgeOp) string {
	parts := []string{op.ID, stampDate(op.At)}
	switch {
	case len(op.Seqs) > 0:
		strs := make([]string, len(op.Seqs))
		for i, s := range op.Seqs {
			strs[i] = strconv.FormatInt(s, 10)
		}
		parts = append(parts, "seq "+strings.Join(strs, ","))
	case op.SeqRange != "":
		parts = append(parts, "seq "+op.SeqRange)
	case op.Seq > 0:
		parts = append(parts, "seq "+strconv.FormatInt(op.Seq, 10))
	}
	if op.Capture != "" {
		parts = append(parts, "capture "+op.Capture)
	}
	if op.Task != "" {
		parts = append(parts, "task "+op.Task)
	}
	if op.Pane != "" {
		parts = append(parts, "pane "+op.Pane)
	}
	if op.Legacy {
		parts = append(parts, "from legacy")
	}
	// The axes, where the lesson lives: kind (with a ? while it is only the migration
	// table's guess), provenance with the observation count, audience once promoted.
	if op.Kind != "" {
		k := op.Kind
		if op.KindAssumed {
			k += "?"
		}
		parts = append(parts, k)
	}
	if op.Provenance != "" {
		p := "from " + op.Provenance
		if op.Hits > 1 {
			p += " ×" + strconv.Itoa(op.Hits)
		}
		parts = append(parts, p)
	}
	if op.Audience != "" {
		parts = append(parts, "for "+op.Audience)
	}
	// The promotion lifecycle is visible where the lesson lives.
	switch {
	case promotionPending(op):
		parts = append(parts, "⚑ promoted (pending)")
	case op.LandedRef != "":
		parts = append(parts, "→ landed "+op.LandedRef)
	}
	return strings.Join(parts, " · ")
}

// isRenderedTopicFile reports whether a file carries the render marker (reads
// only the first line's worth of bytes).
func isRenderedTopicFile(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.HasPrefix(string(b), knowledgeRenderMarker)
}

// migrateTopicFile moves a pre-ledger hand-written topic file out of the render's
// way: verbatim into legacy/ when it holds real content, dropped when it is still
// byte-equal to its seeded placeholder. Idempotent — an absent or already-rendered
// file needs nothing. An occupied legacy slot gets a timestamped sibling rather
// than an overwrite: migration must never destroy bytes.
func migrateTopicFile(topic string, now int64) error {
	path := topicPath(topic)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if strings.HasPrefix(string(b), knowledgeRenderMarker) {
		return nil // already ours
	}
	if seed, ok := TopicSeeds[topic+".md"]; ok && string(b) == seed {
		return os.Remove(path) // an untouched placeholder holds nothing to preserve
	}
	if err := os.MkdirAll(knowledgeLegacyDir(), 0o755); err != nil {
		return err
	}
	dst := filepath.Join(knowledgeLegacyDir(), topic+".md")
	if _, err := os.Stat(dst); err == nil {
		dst = filepath.Join(knowledgeLegacyDir(),
			fmt.Sprintf("%s-%d.md", topic, now))
	}
	return os.Rename(path, dst)
}

// writeTopicRender migrates (first touch) then writes the topic's render.
func writeTopicRender(topic, desc string, live []knowledgeOp, now int64) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	if err := migrateTopicFile(topic, now); err != nil {
		return err
	}
	return os.WriteFile(topicPath(topic), []byte(renderTopic(topic, desc, live)), 0o644)
}

// renderAllTopics re-renders every BUILT-IN topic that has live entries OR
// already carries a rendered file (so a topic whose last entry was retired
// renders empty rather than going stale; untouched seeds stay untouched) — and
// every DECLARED custom topic unconditionally: a declaration is explicit
// intent, and its render (name + description) is what makes it visible.
func renderAllTopics(live, custom []knowledgeOp, now int64) error {
	for _, topic := range BuiltinTopics {
		if !topicHasEntries(live, topic) && !isRenderedTopicFile(topicPath(topic)) {
			continue
		}
		if err := writeTopicRender(topic, "", live, now); err != nil {
			return err
		}
	}
	for _, t := range custom {
		if err := writeTopicRender(t.ID, t.Title, live, now); err != nil {
			return err
		}
	}
	return writeMachineRender(live)
}

// renderMachine is the canonical `machine` file: every live, non-hypothesis entry whose
// audience is this machine, with its exemplar body — the text the agents' index blocks
// point at (phase 3). Rendered even when empty, so the pointer never dangles.
func renderMachine(live []knowledgeOp) string {
	var b strings.Builder
	b.WriteString(knowledgeRenderMarker + "\n")
	b.WriteString("# gtmux · what every agent on this machine must know · 本机所有 agent 都该知道的\n\n")
	n := 0
	for _, op := range live {
		if op.Audience != AudienceMachine || op.Status == StatusHypothesis {
			continue
		}
		renderEntry(&b, op)
		n++
	}
	if n == 0 {
		b.WriteString("_nothing distributed to this machine yet — `gtmux knowledge promote <id> --for machine`_\n")
	}
	return b.String()
}

func writeMachineRender(live []knowledgeOp) error {
	p := MachinePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(renderMachine(live)), 0o644)
}

func topicHasEntries(live []knowledgeOp, topic string) bool {
	for _, op := range live {
		if op.Topic == topic {
			return true
		}
	}
	return false
}

// knowledgeDrift returns the rendered topic files whose on-disk bytes no longer
// match their render — hand edits, which are review material, never silently
// absorbed or overwritten.
func knowledgeDrift(live, custom []knowledgeOp) []string {
	descs := map[string]string{}
	for _, t := range custom {
		descs[t.ID] = t.Title
	}
	var drifted []string
	for _, topic := range knowledgeTopics(custom) {
		path := topicPath(topic)
		if !isRenderedTopicFile(path) {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil || string(b) != renderTopic(topic, descs[topic], live) {
			drifted = append(drifted, path)
		}
	}
	return drifted
}

// stampDate is the date an entry carries, in LOCAL time.
//
// It was UTC, for determinism: the same ledger would render to the same bytes whatever
// the host's timezone. That bought nothing real — this base lives in one machine's home
// and `render --check` regenerates it on that same machine, so local time is every bit as
// deterministic there — and it cost a wrong date every night. East of UTC, everything
// filed between local midnight and 08:00 was stamped with YESTERDAY, silently. Measured
// on 2026-09-01: two entries written at 02:19 and 03:47 both carry 2026-08-31.
//
// The stamp is what a reader consults to tell two entries apart in time (the base says so
// in as many words, after a decay sweep that learned it), so a date that is a day off is
// not cosmetic — it is a live criterion quietly answering wrong.
//
// What UTC would still buy is a memory exported from one timezone and restored in
// another: its first render there rewrites the dates. That is one churn, once, against a
// wrong answer nightly.
func stampDate(unix int64) string {
	return time.Unix(unix, 0).Local().Format("2006-01-02")
}

// audienceWord is the reader-facing word for an audience, both languages.
func audienceWord(a string) string {
	switch a {
	case AudienceHQ:
		return "HQ · this supervisor only"
	case AudienceMachine:
		return "本机 · every agent on this machine"
	case AudienceRepo:
		return "仓库 · agents working in that repository"
	case AudienceEveryone:
		return "全体 · every gtmux user (the product)"
	}
	return ""
}
