// `gtmux capture` — the cheap-notice front of the HQ capture loop (hq-capture-loop ②).
// Writing a polished knowledge-base entry mid-work is too expensive and gets skipped, so
// this decouples NOTICING (one line, in the moment) from WRITING IT UP WELL (batched, at
// distill time). It is a PUBLIC command by design: any worker — not just HQ— that learns
// a durable, cross-cutting fact can drop a CANDIDATE into a pending-distill spool. A
// candidate is NOT a knowledge-base entry: HQ's distill pass is the quality gate that
// decides what is durable, merges it into the right topic (keyed by the dedup key so it
// consolidates instead of scattering near-duplicates), and prunes. Opening the input is
// therefore safe — worst case a candidate is dropped at distill time.
package knowledge

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The capture vocabulary is the knowledge vocabulary (hq-open-topics): built-ins
// plus every ledger-declared topic, judged by the same validKnowledgeTopic the
// knowledge verbs use — one seam, so the two entrances can never drift.

// Candidate is one pending-distill spool line: the lesson + its topic tag + a
// dedup key (so distill MERGES same-key candidates rather than duplicating) + the
// auto-collected event context that gives the distill pass provenance without the author
// re-typing it.
type Candidate struct {
	ID     string `json:"id,omitempty"`
	Digest string `json:"digest,omitempty"` // SHA-256 of retained payload, not a truth score
	At     int64  `json:"at"`               // unix timestamp
	Topic  string `json:"topic"`            // one of captureTopics
	Key    string `json:"key"`              // dedup key: "<topic>/<lesson-slug>"
	Lesson string `json:"lesson"`           // the one-line lesson text
	Pane   string `json:"pane,omitempty"`   // $TMUX_PANE at capture time, if any
	Seq    int64  `json:"seq"`              // the event high-water mark at capture time
	Task   string `json:"task,omitempty"`   // GTMUX_TASK_ID, if the caller is a tracked dispatch
	// Additive (hq-transcript-mining): a candidate the transcript miner queued rather than
	// a person. Source names the miner; Context is the tail of the assistant text a
	// correction answered; Session/Project locate the exchange; Count is a recurring
	// error's tally. All omitted on a `gtmux capture` line.
	Source       string `json:"source,omitempty"`
	Context      string `json:"context,omitempty"`
	Session      string `json:"session,omitempty"`
	Project      string `json:"project,omitempty"`
	Count        int    `json:"count,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Speaker      string `json:"speaker,omitempty"`
	ObservedAt   int64  `json:"observed_at,omitempty"`
	SourceFile   string `json:"source_file,omitempty"`
	SourceOffset int64  `json:"source_offset,omitempty"`
	SourceTurn   string `json:"source_turn,omitempty"`
	// Group is computed on `--list --json` only, never stored: candidates that read as
	// one lesson share a number (1, 2, …); a singleton has none. The screens show a
	// family together and offer one `add --capture k1,k2,…` for it.
	Group int `json:"group,omitempty"`
}

// pendingDistillPath retains observations; ledger settlements define the pending view. It is
// dot-prefixed so it does not clutter the curated knowledge-base topic list.
func pendingDistillPath() string { return filepath.Join(Dir(), ".pending-distill.jsonl") }

// CmdCapture implements `gtmux capture "<lesson> @<topic>"` and `gtmux capture --list
// [--json]`.
// CmdCapture implements the verb; header renders the "is the drain alive?" banner above
// `--list` — it is hq's to compose, because the distill cadence is the supervisor's.
func CmdCapture(args []string, header func(now int64) string) int {
	// A single pass: --list/--json/-h are recognized anywhere; everything else is the
	// lesson. The flags are collected rather than acted on inline so `--list --json` and
	// `--json --list` mean the same thing.
	var rest []string
	list, jsonOut := false, false
	for _, a := range args {
		switch a {
		case "--list", "-l":
			list = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			return captureUsage()
		default:
			rest = append(rest, a)
		}
	}
	if list {
		return captureList(jsonOut, header)
	}

	lesson, topic, ok := parseCaptureInput(rest)
	if !ok {
		i18n.Sae("gtmux capture: need a lesson and a @<topic>",
			"gtmux capture: 需要一句教训和一个 @<topic>")
		captureUsage()
		return 2
	}
	_, custom, err := readKnowledgeState()
	if err != nil {
		i18n.Sae("gtmux capture: "+err.Error(), "gtmux capture: "+err.Error())
		return 1
	}
	if !validKnowledgeTopic(topic, custom) {
		vocab := strings.Join(knowledgeTopics(custom), " | @")
		i18n.Sae("gtmux capture: unknown topic '"+topic+"' (want @"+vocab+"; HQ can declare more with `gtmux knowledge topic`)",
			"gtmux capture: 未知主题 '"+topic+"'（可选 @"+vocab+"；HQ 可用 `gtmux knowledge topic` 声明新主题）")
		return 2
	}

	c := Candidate{
		At:     time.Now().Unix(),
		Topic:  topic,
		Key:    topic + "/" + Slug(lesson),
		Lesson: lesson,
		Pane:   os.Getenv("TMUX_PANE"),
		Seq:    events.LatestSeq(),
		Task:   os.Getenv("GTMUX_TASK_ID"),
	}
	err = AppendCandidate(c)
	// The topic, never the lesson: it is the author's words and waits in the queue.
	diag.Did("act.capture", c.Topic, diag.Outcome(err), "queued a lesson for HQ to judge",
		"bytes", len(lesson), "error", err)
	if err != nil {
		i18n.Sae("gtmux capture: "+err.Error(), "gtmux capture: "+err.Error())
		return 1
	}
	i18n.Say(fmt.Sprintf("captured → %s (%s)", c.Topic, c.Key),
		fmt.Sprintf("已记录 → %s(%s)", c.Topic, c.Key))
	return 0
}

// parseCaptureInput joins the non-flag args and extracts the LAST whitespace-delimited
// `@<topic>` token; the remainder (trimmed) is the lesson. Returns ok=false when either
// the lesson or the topic is missing.
func parseCaptureInput(rest []string) (lesson, topic string, ok bool) {
	joined := strings.TrimSpace(strings.Join(rest, " "))
	if joined == "" {
		return "", "", false
	}
	fields := strings.Fields(joined)
	topicIdx := -1
	for i, f := range fields {
		if strings.HasPrefix(f, "@") && len(f) > 1 {
			topicIdx = i // last @token wins
		}
	}
	if topicIdx < 0 {
		return "", "", false
	}
	topic = strings.TrimPrefix(fields[topicIdx], "@")
	lesson = strings.TrimSpace(strings.Join(append(append([]string{}, fields[:topicIdx]...), fields[topicIdx+1:]...), " "))
	if lesson == "" {
		return "", "", false
	}
	return lesson, topic, true
}

// slug lowercases a lesson and reduces it to a short, stable dedup token: alphanumerics
// kept, every run of other characters becomes a single '-', capped to the first few words
// so two phrasings of the same fact collide on the key.
// untaggedSlug is what a title with no ASCII word produces. For a capture KEY that is a
// fine bucket; for an ENTRY id it is not, and entryID refuses it — see knowledgecmd.go.
const untaggedSlug = "untagged"

func Slug(s string) string {
	var b strings.Builder
	lastDash := true // trim leading dashes
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	// Cap to the first 6 words so an incidental trailing clause doesn't split the key.
	if parts := strings.Split(out, "-"); len(parts) > 6 {
		out = strings.Join(parts[:6], "-")
	}
	if out == "" {
		return untaggedSlug
	}
	return out
}

// AppendCandidate retains the source after settlement. Retrying a source ID is
// idempotent; another observation with the same family key remains separate.
func AppendCandidate(c Candidate) error {
	if c.ID == "" {
		c.ID = diag.NewOpID()
	}
	c.Group = 0
	c.Digest = candidateDigest(c)
	return withKnowledgeLock(func() error {
		all, err := readCandidateSources()
		if err != nil {
			return err
		}
		for _, old := range all {
			if old.ID == c.ID {
				if old.Digest != c.Digest {
					return fmt.Errorf("candidate %s already has different source content", c.ID)
				}
				return nil
			}
		}
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		_, err = atomicAppend(pendingDistillPath(), append(b, '\n'))
		return err
	})
}

func candidateDigest(c Candidate) string {
	c.ID, c.Digest, c.Group = "", "", 0
	b, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// readCandidateSources retains originals. Legacy IDs are derived without a rewrite;
// occurrence numbers distinguish identical legacy observations.
func readCandidateSources() ([]Candidate, error) {
	f, err := os.Open(pendingDistillPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Candidate
	occurrences := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c Candidate
		if json.Unmarshal([]byte(line), &c) == nil {
			digest := candidateDigest(c)
			if c.Digest == "" {
				c.Digest = digest
			}
			if c.ID == "" {
				occurrences[digest]++
				c.ID = fmt.Sprintf("legacy-%s-%d", digest, occurrences[digest])
			}
			out = append(out, c)
		}
	}
	return out, sc.Err()
}

// readCandidates is a projection: only a committed settlement removes an ID.
func readCandidates() ([]Candidate, error) {
	all, err := readCandidateSources()
	if err != nil {
		return nil, err
	}
	ops, err := readKnowledgeOps()
	if err != nil {
		return nil, err
	}
	settled := map[string]bool{}
	for _, op := range ops {
		if op.CandidateResult != "accepted" && op.CandidateResult != "dismissed" {
			continue
		}
		for _, c := range op.Sources {
			settled[c.ID] = true
		}
	}
	out := make([]Candidate, 0, len(all))
	for _, c := range all {
		if !settled[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

// PendingCandidateCount is the spool depth the distill sensor's spool floor reads. An
// unreadable spool counts as 0 — a sensor must never fire on an I/O error.
func PendingCandidateCount() int {
	cands, err := readCandidates()
	if err != nil {
		return 0
	}
	return len(cands)
}

// captureList renders the pending-distill queue, headed by when the queue was last
// DRAINED. The queue depth alone can't tell you whether the loop is alive: an empty queue
// reads identically whether distill drained it yesterday or has never run at all — which
// is exactly how a 13-day distill outage stayed invisible.
func captureList(asJSON bool, header func(now int64) string) int {
	cands, err := readCandidates()
	if err != nil {
		i18n.Sae("gtmux capture: "+err.Error(), "gtmux capture: "+err.Error())
		return 1
	}
	// --json is the read a SURFACE makes. The text form omits the dedup key, which is the
	// one field `gtmux knowledge dismiss --capture <key>` needs, so a GUI could show the
	// queue but never act on it. Always an array, never null: "nothing queued" is a state
	// to render, not a case to special-case.
	if asJSON {
		if cands == nil {
			cands = []Candidate{}
		}
		cands = withFamilies(cands)
		b, err := json.Marshal(cands)
		if err != nil {
			i18n.Sae("gtmux capture: "+err.Error(), "gtmux capture: "+err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	fmt.Println(header(time.Now().Unix()))
	if len(cands) == 0 {
		i18n.Say("pending-distill queue is empty", "待蒸馏队列为空")
		return 0
	}
	i18n.Say(fmt.Sprintf("%d pending-distill candidate(s):", len(cands)),
		fmt.Sprintf("%d 条待蒸馏候选：", len(cands)))
	// Families first: candidates about the same thing are shown together with their
	// keys, so one `knowledge add --capture k1,k2,…` files them as one lesson. The
	// expensive part of draining a queue is seeing which lines are one thing.
	for _, group := range groupCandidates(cands) {
		if len(group) > 1 {
			var keys []string
			for _, c := range group {
				keys = append(keys, c.Key)
			}
			i18n.Say(fmt.Sprintf("  ┌ %d candidates that read as one lesson: knowledge add … --capture %s", len(group), strings.Join(keys, ",")),
				fmt.Sprintf("  ┌ %d 条像是同一件事：knowledge add … --capture %s", len(group), strings.Join(keys, ",")))
		}
		for _, c := range group {
			tag := c.Topic
			if c.Source != "" {
				tag += "·" + c.Source
			}
			indent := "  "
			if len(group) > 1 {
				indent = "  │ "
			}
			fmt.Printf("%s[%s] %s\n", indent, tag, c.Lesson)
			if c.Context != "" {
				fmt.Printf("%s    ↳ %s%s\n", indent, i18n.Tr("after: ", "此前机器说："), c.Context)
			}
		}
	}
	return 0
}

func captureUsage() int {
	i18n.Say("usage: gtmux capture \"<one-line lesson> @<topic>\"   |   gtmux capture --list [--json]",
		"用法：gtmux capture \"<一句话教训> @<topic>\"   |   gtmux capture --list [--json]")
	i18n.Say("  topic ∈ "+strings.Join(BuiltinTopics, " | ")+", plus any topic HQ declared (`gtmux knowledge topic`)",
		"  topic ∈ "+strings.Join(BuiltinTopics, " | ")+"，以及 HQ 用 `gtmux knowledge topic` 声明的主题")
	i18n.Say("  Record a durable, cross-cutting fact as a candidate: cheap, in the moment.",
		"  把一条持久、横向的事实作为候选记下来：便宜，当场。")
	i18n.Say("  Any worker can capture; HQ's distill pass is the quality gate that files it.",
		"  任何 worker 都能记；HQ 的蒸馏回合是把它归档入库的质量闸。")
	return 0
}

// withFamilies returns the pool in family order with Group set on every candidate that
// has company.
func withFamilies(cands []Candidate) []Candidate {
	out := []Candidate{} // never nil: "nothing queued" is a state to render, not null
	family := 0
	for _, g := range groupCandidates(cands) {
		if len(g) > 1 {
			family++
		}
		for _, c := range g {
			if len(g) > 1 {
				c.Group = family
			}
			out = append(out, c)
		}
	}
	return out
}
