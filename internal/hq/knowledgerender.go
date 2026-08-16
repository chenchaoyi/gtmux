// Knowledge RENDERS (hq-knowledge-ledger): each topic markdown file is a
// deterministic render of that topic's live ledger entries — gtmux-owned, marked,
// idempotent, and therefore DRIFT-DETECTABLE: a hand edit is caught by comparing
// the file to its render, not silently absorbed and not silently overwritten.
//
// Migration is incremental: the first mutation touching a topic moves its
// pre-ledger hand-written file VERBATIM to legacy/<topic>.md (a file still equal
// to its seeded placeholder is simply replaced), and the dispatch-time knowledge
// echo consults BOTH, so no lesson loses reach while HQ migrates by use.
package hq

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
func knowledgeLegacyDir() string { return filepath.Join(hqKnowledgeDir(), "legacy") }

func topicPath(topic string) string { return filepath.Join(hqKnowledgeDir(), topic+".md") }

// renderTopic renders one topic's live entries (pure; deterministic — dates in
// UTC so the bytes do not depend on the host timezone).
func renderTopic(topic string, live []knowledgeOp) string {
	var b strings.Builder
	b.WriteString(knowledgeRenderMarker + "\n")
	b.WriteString("# " + topic + "\n\n")
	if _, err := os.Stat(filepath.Join(knowledgeLegacyDir(), topic+".md")); err == nil {
		b.WriteString("> Pre-ledger hand-written entries: [legacy/" + topic + ".md](legacy/" +
			topic + ".md) — migrate the ones you touch.\n\n")
	}
	for _, op := range live {
		if op.Topic != topic {
			continue
		}
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
	return b.String()
}

// provenanceFooter is the one-line evidence trail under each entry.
func provenanceFooter(op knowledgeOp) string {
	parts := []string{op.ID, time.Unix(op.At, 0).UTC().Format("2006-01-02")}
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
	if seed, ok := hqKnowledgeSeeds[topic+".md"]; ok && string(b) == seed {
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
func writeTopicRender(topic string, live []knowledgeOp, now int64) error {
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		return err
	}
	if err := migrateTopicFile(topic, now); err != nil {
		return err
	}
	return os.WriteFile(topicPath(topic), []byte(renderTopic(topic, live)), 0o644)
}

// renderAllTopics re-renders every topic that has live entries OR already carries
// a rendered file (so a topic whose last entry was retired renders empty rather
// than going stale). Topics never touched by the ledger keep their hand-written
// or seeded files untouched.
func renderAllTopics(live []knowledgeOp, now int64) error {
	for _, topic := range knowledgeTopics() {
		if !topicHasEntries(live, topic) && !isRenderedTopicFile(topicPath(topic)) {
			continue
		}
		if err := writeTopicRender(topic, live, now); err != nil {
			return err
		}
	}
	return nil
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
func knowledgeDrift(live []knowledgeOp) []string {
	var drifted []string
	for _, topic := range knowledgeTopics() {
		path := topicPath(topic)
		if !isRenderedTopicFile(path) {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil || string(b) != renderTopic(topic, live) {
			drifted = append(drifted, path)
		}
	}
	return drifted
}
