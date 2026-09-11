package hq

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/mine"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Transcript mining (hq-transcript-mining): the reflector's input to the distill pass.
// gtmux reads the agents' session logs LLM-free, subtracts what the machine wrote, and
// drops correction-shaped exchanges and recurring tool errors into the SAME
// pending-distill spool `gtmux capture` fills — so HQ drains them with the verbs it
// already has, and the spool floor pulls the next distill forward. The miner keeps its
// own ledger (offsets, emitted ids, error tally) so a daily pass never reads or emits
// anything twice.

const (
	captureSourceTranscript = "transcript"
	// mineDefaultWindow bounds CORRECTION candidates on a first run: the stock of old
	// sessions would otherwise land as hundreds of spool lines nobody drains by hand.
	// `gtmux knowledge mine --since all` is the explicit door to the stock.
	mineDefaultWindow = 30 * 24 * time.Hour
)

func mineDir() string { return filepath.Join(state.Dir(), "mine") }

// mineSensor runs a pass from the serve slow tick once per interval, only when an HQ
// home exists (the spool lives there). It raises no wake: the spool floor is the trigger.
func mineSensor(now int64) {
	hours := hqwake.Load().MineIntervalHours
	if hours <= 0 {
		return
	}
	if _, err := os.Stat(hqKnowledgeDir()); err != nil {
		return
	}
	if last := mine.LastPassAt(mineDir()); last != 0 && now-last < hours*3600 {
		return
	}
	_, _ = runMinePass(mine.Options{Now: time.Unix(now, 0), Since: mineSince(now, mineDefaultWindow)})
}

// mineSince is the correction window: a bound on the first pass; later passes are already
// bounded by the ledger offsets, but the window still applies so a long-dormant log
// resumed today cannot drag months of old reactions in.
func mineSince(now int64, window time.Duration) time.Time {
	if window <= 0 {
		return time.Time{}
	}
	return time.Unix(now, 0).Add(-window)
}

// runMinePass runs the miner with the audit journal's send heads as the machine set and
// appends every new candidate to the spool. A dry run appends nothing.
func runMinePass(o mine.Options) (mine.Report, error) {
	o.MachineHeads = machineHeads()
	rep, err := mine.Run(mineDir(), o)
	if err != nil || o.DryRun {
		return rep, err
	}
	seq := events.CurrentSeq()
	for _, c := range rep.Candidates {
		if err := appendCandidate(spoolFromMined(c, rep.At, seq)); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// machineHeads is every `gtmux send` payload head the audit journal still holds. The
// journal is size-rotated, so a send older than the log is simply not subtracted — the
// lexicon then decides, and HQ reads the survivor like any other lead.
func machineHeads() map[string]bool {
	recs, _ := events.ReadSince(0)
	var sums []string
	for _, r := range recs {
		if r.Event == events.AuditEventSend {
			sums = append(sums, r.Summary)
		}
	}
	return mine.HeadsFromSendSummaries(sums)
}

// spoolFromMined maps a lead onto the spool line shape. Topic: a correction-shaped
// exchange is `corrections` material; a recurring error is a `pitfalls` lead. The key
// carries the stable id so `knowledge add --capture <key>` / `dismiss --capture <key>`
// address exactly one lead.
func spoolFromMined(c mine.Candidate, at, seq int64) captureCandidate {
	cc := captureCandidate{
		At: at, Seq: seq, Source: captureSourceTranscript,
		Session: c.Session, Project: c.Project, Context: c.Context, Count: c.Count,
	}
	switch c.Kind {
	case mine.KindError:
		cc.Topic = "pitfalls"
		cc.Key = "pitfalls/mined-" + slug(c.Line)
		cc.Lesson = fmt.Sprintf("%s (×%d, %d sessions)", c.Line, c.Count, c.Sessions)
	default:
		cc.Topic = "corrections"
		cc.Key = "corrections/mined-" + c.ID
		cc.Lesson = c.Line
	}
	return cc
}

// knowledgeMine implements `gtmux knowledge mine [--dry-run] [--since <dur>|all] [--json]
// [--status]`. It runs from anywhere: the spool is the open door `gtmux capture` already
// is, and the ledger is the miner's own.
func knowledgeMine(args []string) int {
	var dry, asJSON, status bool
	window := mineDefaultWindow
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run", "-n":
			dry = true
		case "--json":
			asJSON = true
		case "--status":
			status = true
		case "--since":
			if i+1 >= len(args) {
				i18n.Sae("gtmux knowledge mine: --since needs a duration (e.g. 7d, 90d) or 'all'",
					"gtmux knowledge mine: --since 需要一个时长（如 7d、90d）或 'all'")
				return 2
			}
			i++
			d, ok := parseSinceArg(args[i])
			if !ok {
				i18n.Sae("gtmux knowledge mine: bad --since "+args[i], "gtmux knowledge mine: --since 无法识别 "+args[i])
				return 2
			}
			window = d
		case "-h", "--help":
			return mineUsage()
		default:
			i18n.Sae("gtmux knowledge mine: unknown flag "+args[i], "gtmux knowledge mine: 未知参数 "+args[i])
			return 2
		}
	}
	if status {
		return mineStatus(asJSON)
	}
	now := time.Now()
	rep, err := runMinePass(mine.Options{Now: now, Since: mineSince(now.Unix(), window), DryRun: dry})
	if err != nil {
		i18n.Sae("gtmux knowledge mine: "+err.Error(), "gtmux knowledge mine: "+err.Error())
		return 1
	}
	if asJSON {
		b, _ := json.Marshal(rep)
		fmt.Println(string(b))
		return 0
	}
	verb := i18n.Tr("queued", "已入队")
	if dry {
		verb = i18n.Tr("would queue (dry run)", "将入队（试运行）")
	}
	fmt.Println(i18n.Tr(
		fmt.Sprintf("read %d file(s), %s, %d human line(s) → %d candidate(s) %s, %d already emitted",
			rep.Files, humanBytes(rep.Bytes), rep.Lines, len(rep.Candidates), verb, rep.Skipped),
		fmt.Sprintf("读了 %d 个文件、%s、%d 条人类发言 → %d 条候选%s，%d 条此前已发过",
			rep.Files, humanBytes(rep.Bytes), rep.Lines, len(rep.Candidates), verb, rep.Skipped)))
	for _, c := range rep.Candidates {
		fmt.Println("  " + renderMined(c))
	}
	return 0
}

func renderMined(c mine.Candidate) string {
	switch c.Kind {
	case mine.KindError:
		return fmt.Sprintf("[pitfalls] %s  (×%d, %d sessions)", c.Line, c.Count, c.Sessions)
	default:
		s := fmt.Sprintf("[corrections] %s", c.Line)
		if c.Context != "" {
			s += "\n      ↳ " + i18n.Tr("after: ", "此前机器说：") + c.Context
		}
		return s
	}
}

func mineStatus(asJSON bool) int {
	st := mine.ReadStatus(mineDir(), 10)
	if asJSON {
		b, _ := json.Marshal(st)
		fmt.Println(string(b))
		return 0
	}
	if st.LastPass == 0 {
		i18n.Say("transcript mining: never run", "会话采矿：从未运行")
		return 0
	}
	age := HumanAgeShort(time.Now().Unix() - st.LastPass)
	fmt.Println(i18n.Tr(
		fmt.Sprintf("last pass %s ago · %d passes · %d sources · %d candidates emitted · %d error signatures",
			age, st.Passes, st.Sources, st.Emitted, st.Errors),
		fmt.Sprintf("上次采矿 %s前 · 共 %d 轮 · %d 个来源 · 已发 %d 条候选 · %d 种报错签名",
			age, st.Passes, st.Sources, st.Emitted, st.Errors)))
	if len(st.TopErrors) > 0 {
		i18n.Say("recurring errors (sessions × count):", "反复出现的报错（会话数 × 次数）：")
		for _, e := range st.TopErrors {
			mark := " "
			if e.Emitted {
				mark = "✓"
			}
			fmt.Printf("  %s %2d × %-4d %s\n", mark, e.Sessions, e.Count, e.Line)
		}
	}
	return 0
}

func mineUsage() int {
	i18n.Say(`usage: gtmux knowledge mine [--dry-run] [--since <Nd>|all] [--json]
       gtmux knowledge mine --status [--json]
  Mine the agents' session logs (LLM-free) for correction-shaped exchanges and recurring
  tool errors, and queue them as pending-distill candidates for HQ to judge. Incremental:
  a ledger under the state dir keeps offsets and emitted ids, so nothing is read or queued
  twice. --since bounds the correction window (default 30d; 'all' for the whole stock).`,
		`用法：gtmux knowledge mine [--dry-run] [--since <N>d|all] [--json]
      gtmux knowledge mine --status [--json]
  不用模型，从各 agent 的会话日志里挖「纠正形状的对话」和「反复出现的报错」，投进待蒸馏
  队列交给 HQ 判断。增量式：状态目录下的台账记着偏移和已发的 id，不会重复读、重复投。
  --since 限定纠正候选的时间窗（默认 30 天；'all' 挖全部存量）。`)
	return 0
}

// parseSinceArg accepts "all", "<N>d", "<N>h".
func parseSinceArg(s string) (time.Duration, bool) {
	if s == "all" {
		return 0, true
	}
	unit := time.Hour * 24
	switch {
	case strings.HasSuffix(s, "d"):
		s = strings.TrimSuffix(s, "d")
	case strings.HasSuffix(s, "h"):
		s, unit = strings.TrimSuffix(s, "h"), time.Hour
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return time.Duration(n) * unit, true
}
