// `gtmux knowledge` — the supervisor's write path into the knowledge ledger
// (hq-knowledge-ledger). Mutations are gated to the HQ home by the same cwd-keyed
// role rule as `gtmux events --ack`: the quality gate is the supervisor, and a
// worker's input stays `gtmux capture`. Every mutation appends one ledger
// operation, re-renders the affected topic files, and journals one
// `gtmux:audit:knowledge` record — so the base's change history is a stream
// query, not an archaeology dig.
package hq

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// CmdKnowledge implements `gtmux knowledge <add|supersede|retire|dismiss|list|show|render>`.
func CmdKnowledge(args []string) int {
	if len(args) == 0 {
		return knowledgeUsage()
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "add":
		return knowledgeMutation(func() error { return knowledgeAdd(rest) })
	case "supersede":
		return knowledgeMutation(func() error { return knowledgeSupersede(rest) })
	case "retire":
		return knowledgeMutation(func() error { return knowledgeRetire(rest) })
	case "dismiss":
		return knowledgeMutation(func() error { return knowledgeDismiss(rest) })
	case "render":
		return knowledgeMutation(func() error { return knowledgeRender(rest) })
	case "promote":
		return knowledgeMutation(func() error { return knowledgePromote(rest) })
	case "land":
		return knowledgeMutation(func() error { return knowledgeLand(rest) })
	case "promotions":
		return knowledgePromotions(rest)
	case "list":
		return knowledgeList(rest)
	case "show":
		return knowledgeShow(rest)
	case "-h", "--help":
		return knowledgeUsage()
	default:
		i18n.Sae("gtmux knowledge: unknown verb '"+verb+"'", "gtmux knowledge: 未知子命令 '"+verb+"'")
		return 2
	}
}

// knowledgeMutation is the shared mutation wrapper: the HQ-home role gate first
// (loud, mirroring `events --ack`), then the verb, with its error reported once.
func knowledgeMutation(run func() error) int {
	if !fromHQHome() {
		i18n.Sae("gtmux knowledge: only the HQ session can write knowledge (run it from "+hqHomeForMessage()+"); workers record candidates with `gtmux capture`",
			"gtmux knowledge: 只有中控会话能写知识库（请在 "+hqHomeForMessage()+" 下运行）；worker 请用 `gtmux capture` 记候选")
		return 1
	}
	if err := run(); err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 1
	}
	return 0
}

// hqHomeForMessage names the HQ home in refusal messages.
func hqHomeForMessage() string { return state.HQHome() }

// knowledgeFlags is the shared flag set of the content-carrying verbs.
type knowledgeFlags struct {
	topic, title, bodyFile, capture, seqRange, why string
	target, ref                                    string
	jsonOut                                        bool
	positional                                     []string
}

func parseKnowledgeFlags(args []string) (knowledgeFlags, error) {
	var f knowledgeFlags
	take := func(i *int, name string) (string, error) {
		if *i+1 >= len(args) {
			return "", fmt.Errorf("%s needs a value", name)
		}
		*i++
		return args[*i], nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		var err error
		switch {
		case a == "--topic":
			f.topic, err = take(&i, a)
		case strings.HasPrefix(a, "--topic="):
			f.topic = strings.TrimPrefix(a, "--topic=")
		case a == "--title":
			f.title, err = take(&i, a)
		case strings.HasPrefix(a, "--title="):
			f.title = strings.TrimPrefix(a, "--title=")
		case a == "--body-file":
			f.bodyFile, err = take(&i, a)
		case strings.HasPrefix(a, "--body-file="):
			f.bodyFile = strings.TrimPrefix(a, "--body-file=")
		case a == "--capture":
			f.capture, err = take(&i, a)
		case strings.HasPrefix(a, "--capture="):
			f.capture = strings.TrimPrefix(a, "--capture=")
		case a == "--seq-range":
			f.seqRange, err = take(&i, a)
		case strings.HasPrefix(a, "--seq-range="):
			f.seqRange = strings.TrimPrefix(a, "--seq-range=")
		case a == "--why":
			f.why, err = take(&i, a)
		case strings.HasPrefix(a, "--why="):
			f.why = strings.TrimPrefix(a, "--why=")
		case a == "--target":
			f.target, err = take(&i, a)
		case strings.HasPrefix(a, "--target="):
			f.target = strings.TrimPrefix(a, "--target=")
		case a == "--ref":
			f.ref, err = take(&i, a)
		case strings.HasPrefix(a, "--ref="):
			f.ref = strings.TrimPrefix(a, "--ref=")
		case a == "--json":
			f.jsonOut = true
		case strings.HasPrefix(a, "--"):
			return f, fmt.Errorf("unknown option '%s'", a)
		default:
			f.positional = append(f.positional, a)
		}
		if err != nil {
			return f, err
		}
	}
	return f, nil
}

// readBody loads --body-file (path or `-` for stdin) through the shared
// shell-free payload channel — the dispatch-file-channel lesson: prose as argv
// must survive a shell first, so it never rides argv here.
func readBody(bodyFile string) (string, error) {
	if bodyFile == "" {
		return "", nil
	}
	return dispatch.ReadPayload(bodyFile, os.Stdin)
}

// parseSeqRange validates "a..b" (both positive, a ≤ b).
func parseSeqRange(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	parts := strings.SplitN(s, "..", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("--seq-range wants a..b, got %q", s)
	}
	a, err1 := strconv.ParseInt(parts[0], 10, 64)
	b, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || a <= 0 || b < a {
		return "", fmt.Errorf("--seq-range wants a..b with 0 < a ≤ b, got %q", s)
	}
	return s, nil
}

func knowledgeAdd(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if f.topic == "" || f.title == "" {
		return fmt.Errorf("add needs --topic and --title")
	}
	if !validKnowledgeTopic(f.topic) {
		return fmt.Errorf("unknown topic %q (want %s)", f.topic, strings.Join(knowledgeTopics(), " | "))
	}
	body, err := readBody(f.bodyFile)
	if err != nil {
		return err
	}
	seqRange, err := parseSeqRange(f.seqRange)
	if err != nil {
		return err
	}
	op := knowledgeOp{
		Op: knowledgeOpAdd, ID: f.topic + "/" + slug(f.title), Topic: f.topic,
		Title: f.title, Body: body, At: time.Now().Unix(),
		Seq: events.LatestSeq(), SeqRange: seqRange,
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	if _, exists := findLive(live, op.ID); exists {
		return fmt.Errorf("id %s is a live entry — `gtmux knowledge supersede %s --title …` to replace it, or retitle", op.ID, op.ID)
	}
	auditNote := "add " + op.ID
	if f.capture != "" {
		consumed, err := consumeCandidates(f.capture)
		if err != nil {
			return err
		}
		op.Capture = f.capture
		for _, c := range consumed {
			if c.Seq > 0 {
				op.Seqs = append(op.Seqs, c.Seq)
			}
		}
		newest := consumed[len(consumed)-1]
		op.Pane, op.Task = newest.Pane, newest.Task
		auditNote += fmt.Sprintf(" (capture %s ×%d)", f.capture, len(consumed))
	}
	return commitKnowledgeOp(op, auditNote)
}

func knowledgeSupersede(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.title == "" {
		return fmt.Errorf("supersede needs <id> and --title")
	}
	predID := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	pred, ok := findLive(live, predID)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", predID)
	}
	body, err := readBody(f.bodyFile)
	if err != nil {
		return err
	}
	op := knowledgeOp{
		Op: knowledgeOpSupersede, Supersedes: predID,
		ID: pred.Topic + "/" + slug(f.title), Topic: pred.Topic,
		Title: f.title, Body: body, At: time.Now().Unix(),
		Seq: events.LatestSeq(), Why: f.why,
	}
	if op.ID != predID {
		if _, exists := findLive(live, op.ID); exists {
			return fmt.Errorf("retitled id %s collides with a live entry — pick another title", op.ID)
		}
	}
	return commitKnowledgeOp(op, "supersede "+predID+" → "+op.ID)
}

func knowledgeRetire(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.why == "" {
		return fmt.Errorf("retire needs <id> and --why (the reason survives; make it worth reading)")
	}
	id := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	op := knowledgeOp{
		Op: knowledgeOpRetire, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(), Why: f.why,
	}
	return commitKnowledgeOp(op, "retire "+id+": "+f.why)
}

// knowledgePromote opens the export lifecycle on a live entry: the mechanical
// form of "FLAG it for a seed/spec update" (hq-promotion-exit). The commit path
// writes the brief via renderPromotions.
func knowledgePromote(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.why == "" {
		return fmt.Errorf("promote needs <id> and --why (the promotion case — it survives into the brief)")
	}
	id := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	if promotionPending(entry) {
		return fmt.Errorf("%s is already promoted and pending — land it (`gtmux knowledge land %s --ref …`) before promoting again", id, id)
	}
	op := knowledgeOp{
		Op: knowledgeOpPromote, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(),
		Why: f.why, Target: f.target,
	}
	return commitKnowledgeOp(op, "promote "+id)
}

// knowledgeLand closes a pending promotion with the repo reference; the commit
// path's promotion sweep removes the brief.
func knowledgeLand(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.ref == "" {
		return fmt.Errorf("land needs <id> and --ref (where it landed: a PR, a spec, a seed change)")
	}
	id := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	if !promotionPending(entry) {
		return fmt.Errorf("%s has no pending promotion to land (gtmux knowledge promotions)", id)
	}
	op := knowledgeOp{
		Op: knowledgeOpLand, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(), Ref: f.ref,
	}
	return commitKnowledgeOp(op, "land "+id+" → "+f.ref)
}

// knowledgePromotions lists the pending export queue — read-only, open to
// anyone, HEADED by the count and the oldest age so rot is visible at a glance
// (the instrument a local flags file never had).
func knowledgePromotions(args []string) int {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 2
	}
	live, err := liveKnowledge()
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 1
	}
	pending, oldestAt := pendingPromotions(live)
	if f.jsonOut {
		if pending == nil {
			pending = []knowledgeOp{}
		}
		b, _ := json.Marshal(pending)
		fmt.Println(string(b))
		return 0
	}
	if len(pending) == 0 {
		i18n.Say("no pending promotions — the exit queue is clear", "无待落地晋升 —— 出口队列已清空")
		return 0
	}
	now := time.Now().Unix()
	i18n.Say(fmt.Sprintf("%d pending promotion(s), oldest %s ago:",
		len(pending), HumanAgeShort(now-oldestAt)),
		fmt.Sprintf("%d 条待落地晋升,最久 %s 前:", len(pending), HumanAgeShort(now-oldestAt)))
	for _, op := range pending {
		fmt.Printf("  %-40s  %s  (%s)\n", op.ID, HumanAgeShort(now-op.PromotedAt), promotionBriefPath(op))
	}
	return 0
}

// knowledgeDismiss rejects a pending capture candidate WITH a trace: no ledger
// operation, but the spool line is gone and the journal says why — the quality
// gate's rejections stop vanishing identically to its acceptances.
func knowledgeDismiss(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if f.capture == "" || f.why == "" {
		return fmt.Errorf("dismiss needs --capture <key> and --why")
	}
	if err := validateKnowledgeContent("", "", f.why); err != nil {
		return err
	}
	consumed, err := consumeCandidates(f.capture)
	if err != nil {
		return err
	}
	events.AuditKnowledge(fmt.Sprintf("dismiss %s ×%d: %s", f.capture, len(consumed), f.why),
		time.Now().Unix())
	i18n.Say(fmt.Sprintf("dismissed %d candidate(s) under %s", len(consumed), f.capture),
		fmt.Sprintf("已驳回 %d 条候选（%s）", len(consumed), f.capture))
	return nil
}

// commitKnowledgeOp is the shared mutation tail: append, re-render, journal.
func commitKnowledgeOp(op knowledgeOp, auditNote string) error {
	if err := appendKnowledgeOp(op); err != nil {
		return err
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	if err := renderAllTopics(live, time.Now().Unix()); err != nil {
		return err
	}
	if err := renderPromotions(live); err != nil {
		return err
	}
	events.AuditKnowledge(auditNote, time.Now().Unix())
	i18n.Say("✓ "+auditNote, "✓ "+auditNote)
	return nil
}

func knowledgeRender(args []string) error {
	check := false
	for _, a := range args {
		switch a {
		case "--check":
			check = true
		default:
			return fmt.Errorf("unknown option '%s'", a)
		}
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	if check {
		drifted := knowledgeDrift(live)
		if len(drifted) == 0 {
			return nil
		}
		for _, p := range drifted {
			i18n.Sae("drift: "+p+" no longer matches its render — hand edits go through `gtmux knowledge`; `gtmux knowledge render` to restore",
				"drift: "+p+" 与生成结果不一致 —— 手改请走 `gtmux knowledge`；`gtmux knowledge render` 可恢复")
		}
		return fmt.Errorf("%d rendered file(s) drifted", len(drifted))
	}
	return renderAllTopics(live, time.Now().Unix())
}

func knowledgeList(args []string) int {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 2
	}
	live, err := liveKnowledge()
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 1
	}
	var out []knowledgeOp
	for _, op := range live {
		if f.topic == "" || op.Topic == f.topic {
			out = append(out, op)
		}
	}
	if f.jsonOut {
		b, _ := json.Marshal(out)
		fmt.Println(string(b))
		return 0
	}
	if len(out) == 0 {
		i18n.Say("no live entries", "暂无有效条目")
		return 0
	}
	for _, op := range out {
		fmt.Printf("%-40s  %s\n", op.ID, op.Title)
	}
	return 0
}

func knowledgeShow(args []string) int {
	if len(args) != 1 {
		i18n.Sae("usage: gtmux knowledge show <id>", "用法：gtmux knowledge show <id>")
		return 2
	}
	live, err := liveKnowledge()
	if err != nil {
		i18n.Sae("gtmux knowledge: "+err.Error(), "gtmux knowledge: "+err.Error())
		return 1
	}
	op, ok := findLive(live, args[0])
	if !ok {
		i18n.Sae("gtmux knowledge: no live entry '"+args[0]+"'", "gtmux knowledge: 找不到有效条目 '"+args[0]+"'")
		return 1
	}
	fmt.Println("# " + op.Title)
	if strings.TrimSpace(op.Body) != "" {
		fmt.Println("\n" + op.Body)
	}
	fmt.Println("\n· " + provenanceFooter(op))
	return 0
}

func knowledgeUsage() int {
	i18n.Say(`usage: gtmux knowledge <verb>
  add       --topic <t> --title "<one line>" [--body-file <path|->] [--capture <key>] [--seq-range a..b]
  supersede <id> --title "<one line>" [--body-file <path|->] [--why "<reason>"]
  retire    <id> --why "<reason>"
  dismiss   --capture <key> --why "<reason>"
  promote   <id> --why "<case>" [--target "<repo spot>"]   # charter-level → export brief
  land      <id> --ref "<pr/spec>"                         # close the loop when it lands
  promotions [--json]                                      # the pending export queue
  list      [--topic <t>] [--json]     show <id>     render [--check]
  The knowledge base's authority is an append-only ledger; topic .md files are
  rendered from it, entries carry provenance (seq/pane/task/capture), and every
  mutation is journaled. A charter-level lesson exits through promote → a brief
  under knowledge/promotions/ → land. Mutations run from the HQ home only;
  workers use `+"`gtmux capture`"+`.`,
		`用法：gtmux knowledge <子命令>
  add       --topic <主题> --title "<一句话>" [--body-file <路径|->] [--capture <键>] [--seq-range a..b]
  supersede <id> --title "<一句话>" [--body-file <路径|->] [--why "<原因>"]
  retire    <id> --why "<原因>"
  dismiss   --capture <键> --why "<原因>"
  promote   <id> --why "<理由>" [--target "<落点>"]   # 守则级 → 生成外送简报
  land      <id> --ref "<pr/spec>"                    # 落地后闭环
  promotions [--json]                                 # 待落地队列
  list      [--topic <主题>] [--json]     show <id>     render [--check]
  知识库以追加式台账为准，主题 .md 由它生成；条目携带来源证据（seq/pane/task/capture），
  每次变更都写入事件流。守则级教训经 promote 生成 knowledge/promotions/ 下的简报,
  落地后用 land 闭环。变更只能在中控目录执行；worker 用 `+"`gtmux capture`"+`。`)
	return 0
}
