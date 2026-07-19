// `gtmux capture` — the cheap-notice front of the HQ capture loop (hq-capture-loop ②).
// Writing a polished knowledge-base entry mid-work is too expensive and gets skipped, so
// this decouples NOTICING (one line, in the moment) from WRITING IT UP WELL (batched, at
// distill time). It is a PUBLIC command by design: any worker — not just HQ — that learns
// a durable, cross-cutting fact can drop a CANDIDATE into a pending-distill spool. A
// candidate is NOT a knowledge-base entry: HQ's distill pass is the quality gate that
// decides what is durable, merges it into the right topic (keyed by the dedup key so it
// consolidates instead of scattering near-duplicates), and prunes. Opening the input is
// therefore safe — worst case a candidate is dropped at distill time.
package hq

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// captureTopics is the fixed topic vocabulary a candidate may be filed under — the same
// curated knowledge-base topics HQ maintains (README + environment are not capture
// targets: the former is an index, the latter is machine-specific config).
var captureTopics = []string{"accounts", "workflows", "best-practices", "pitfalls", "corrections"}

// captureCandidate is one pending-distill spool line: the lesson + its topic tag + a
// dedup key (so distill MERGES same-key candidates rather than duplicating) + the
// auto-collected event context that gives the distill pass provenance without the author
// re-typing it.
type captureCandidate struct {
	At     int64  `json:"at"`             // unix timestamp
	Topic  string `json:"topic"`          // one of captureTopics
	Key    string `json:"key"`            // dedup key: "<topic>/<lesson-slug>"
	Lesson string `json:"lesson"`         // the one-line lesson text
	Pane   string `json:"pane,omitempty"` // $TMUX_PANE at capture time, if any
	Seq    int64  `json:"seq"`            // the event high-water mark at capture time
	Task   string `json:"task,omitempty"` // GTMUX_TASK_ID, if the caller is a tracked dispatch
}

// pendingDistillPath is the append-only spool the distill pass drains + truncates. It is
// dot-prefixed so it does not clutter the curated knowledge-base topic list.
func pendingDistillPath() string { return filepath.Join(hqKnowledgeDir(), ".pending-distill.jsonl") }

// CmdCapture implements `gtmux capture "<lesson> @<topic>"` and `gtmux capture --list`.
func CmdCapture(args []string) int {
	// A single pass: --list/-h are recognized anywhere; everything else is the lesson.
	var rest []string
	for _, a := range args {
		switch a {
		case "--list", "-l":
			return captureList()
		case "-h", "--help":
			return captureUsage()
		default:
			rest = append(rest, a)
		}
	}

	lesson, topic, ok := parseCaptureInput(rest)
	if !ok {
		i18n.Sae("gtmux capture: need a lesson and a @<topic>",
			"gtmux capture: 需要一句教训和一个 @<topic>")
		captureUsage()
		return 2
	}
	if !validCaptureTopic(topic) {
		i18n.Sae("gtmux capture: unknown topic '"+topic+"' (want @"+strings.Join(captureTopics, " | @")+")",
			"gtmux capture: 未知主题 '"+topic+"'(可选 @"+strings.Join(captureTopics, " | @")+")")
		return 2
	}

	c := captureCandidate{
		At:     time.Now().Unix(),
		Topic:  topic,
		Key:    topic + "/" + slug(lesson),
		Lesson: lesson,
		Pane:   os.Getenv("TMUX_PANE"),
		Seq:    events.LatestSeq(),
		Task:   os.Getenv("GTMUX_TASK_ID"),
	}
	if err := appendCandidate(c); err != nil {
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

func validCaptureTopic(topic string) bool {
	for _, t := range captureTopics {
		if t == topic {
			return true
		}
	}
	return false
}

// slug lowercases a lesson and reduces it to a short, stable dedup token: alphanumerics
// kept, every run of other characters becomes a single '-', capped to the first few words
// so two phrasings of the same fact collide on the key.
func slug(s string) string {
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
		return "untagged"
	}
	return out
}

// appendCandidate appends one JSON line to the spool, creating the knowledge dir if needed.
func appendCandidate(c captureCandidate) error {
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(pendingDistillPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// readCandidates loads the spool (empty slice when absent).
func readCandidates() ([]captureCandidate, error) {
	f, err := os.Open(pendingDistillPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []captureCandidate
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c captureCandidate
		if json.Unmarshal([]byte(line), &c) == nil {
			out = append(out, c)
		}
	}
	return out, sc.Err()
}

// captureList renders the pending-distill queue.
func captureList() int {
	cands, err := readCandidates()
	if err != nil {
		i18n.Sae("gtmux capture: "+err.Error(), "gtmux capture: "+err.Error())
		return 1
	}
	if len(cands) == 0 {
		i18n.Say("pending-distill queue is empty", "待蒸馏队列为空")
		return 0
	}
	i18n.Say(fmt.Sprintf("%d pending-distill candidate(s):", len(cands)),
		fmt.Sprintf("%d 条待蒸馏候选:", len(cands)))
	for _, c := range cands {
		fmt.Printf("  [%s] %s\n", c.Topic, c.Lesson)
	}
	return 0
}

func captureUsage() int {
	i18n.Say("usage: gtmux capture \"<one-line lesson> @<topic>\"   |   gtmux capture --list",
		"用法：gtmux capture \"<一句话教训> @<topic>\"   |   gtmux capture --list")
	i18n.Say("  topic ∈ "+strings.Join(captureTopics, " | "),
		"  topic ∈ "+strings.Join(captureTopics, " | "))
	i18n.Say("  Record a durable, cross-cutting fact as a CANDIDATE — cheap, in the moment.",
		"  把一条持久、横向的事实作为候选记下来 —— 便宜、当场。")
	i18n.Say("  Any worker can capture; HQ's distill pass is the quality gate that files it.",
		"  任何 worker 都能记;HQ 的蒸馏回合是把它归档入库的质量闸。")
	return 0
}
