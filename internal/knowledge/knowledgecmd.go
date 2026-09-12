// `gtmux knowledge` — the supervisor's write path into the knowledge ledger
// (hq-knowledge-ledger). Mutations are gated to the HQ home by the same cwd-keyed
// role rule as `gtmux events --ack`: the quality gate is the supervisor, and a
// worker's input stays `gtmux capture`. Every mutation appends one ledger
// operation, re-renders the affected topic files, and journals one
// `gtmux:audit:knowledge` record — so the base's change history is a stream
// query, not an archaeology dig.
package knowledge

import (
	"encoding/json"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/humanize"
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
	case "withdraw":
		return knowledgeMutation(func() error { return knowledgeWithdraw(rest) })
	case "sync":
		return knowledgeSync(rest)
	case "carriers":
		return knowledgeCarriers(rest)
	case "topic":
		return knowledgeMutation(func() error { return knowledgeTopic(rest) })
	case "kind":
		return knowledgeMutation(func() error { return knowledgeKind(rest) })
	case "hit":
		return knowledgeMutation(func() error { return knowledgeHit(rest) })
	case "confirm":
		return knowledgeMutation(func() error { return knowledgeConfirm(rest) })
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
			"gtmux knowledge: 只有 HQ 会话能写知识库（请在 "+hqHomeForMessage()+" 下运行）；worker 请用 `gtmux capture` 记候选")
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

// fromHQHome is the cwd-keyed role rule: only a process whose cwd IS the HQ home writes
// knowledge (the same rule the radar and the pull stamp use).
func fromHQHome() bool {
	cwd, err := os.Getwd()
	return err == nil && cwd == state.HQHome()
}

// knowledgeFlags is the shared flag set of the content-carrying verbs.
type knowledgeFlags struct {
	topic, title, bodyFile, seqRange, why string
	target, ref, descText                 string
	// captures: every --capture key given (repeatable, or comma-separated). Same-family
	// candidates are ONE lesson; consuming them into one entry keeps every provenance,
	// where accepting one and dismissing ten scattered it across dismissal records.
	captures   []string
	jsonOut    bool
	positional []string
	// The axes on add / supersede / list: --kind, --tags, --provenance, --hypothesis.
	kind, provenance string
	tags             []string
	hypothesis       bool
	n                int
	// --for <hq|machine|repo:<path>|everyone> on promote; --force on sync / land.
	audience, audienceRepo string
	force                  bool
	repo                   string
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
			var v string
			v, err = take(&i, a)
			f.captures = append(f.captures, splitKeys(v)...)
		case strings.HasPrefix(a, "--capture="):
			f.captures = append(f.captures, splitKeys(strings.TrimPrefix(a, "--capture="))...)
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
		case a == "--desc":
			f.descText, err = take(&i, a)
		case strings.HasPrefix(a, "--desc="):
			f.descText = strings.TrimPrefix(a, "--desc=")
		case a == "--kind":
			f.kind, err = take(&i, a)
		case strings.HasPrefix(a, "--kind="):
			f.kind = strings.TrimPrefix(a, "--kind=")
		case a == "--provenance":
			f.provenance, err = take(&i, a)
		case strings.HasPrefix(a, "--provenance="):
			f.provenance = strings.TrimPrefix(a, "--provenance=")
		case a == "--tags":
			var v string
			v, err = take(&i, a)
			f.tags = append(f.tags, splitKeys(v)...)
		case strings.HasPrefix(a, "--tags="):
			f.tags = append(f.tags, splitKeys(strings.TrimPrefix(a, "--tags="))...)
		case a == "--hypothesis":
			f.hypothesis = true
		case a == "--force":
			f.force = true
		case a == "--repo":
			f.repo, err = take(&i, a)
		case strings.HasPrefix(a, "--repo="):
			f.repo = strings.TrimPrefix(a, "--repo=")
		case a == "--for":
			var v string
			v, err = take(&i, a)
			if err == nil {
				f.audience, f.audienceRepo, err = parseAudience(v)
			}
		case strings.HasPrefix(a, "--for="):
			f.audience, f.audienceRepo, err = parseAudience(strings.TrimPrefix(a, "--for="))
		case a == "--n":
			var v string
			v, err = take(&i, a)
			if err == nil {
				f.n, err = strconv.Atoi(v)
			}
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
	live, custom, err := readKnowledgeState()
	if err != nil {
		return err
	}
	if !validKnowledgeTopic(f.topic, custom) {
		return fmt.Errorf("unknown topic %q (want %s; declare your own with `gtmux knowledge topic <name> --desc …`)",
			f.topic, strings.Join(knowledgeTopics(custom), " | "))
	}
	body, err := readBody(f.bodyFile)
	if err != nil {
		return err
	}
	seqRange, err := parseSeqRange(f.seqRange)
	if err != nil {
		return err
	}
	addID, err := entryID(f.topic, f.title)
	if err != nil {
		return err
	}
	op := knowledgeOp{
		Op: knowledgeOpAdd, ID: addID, Topic: f.topic,
		Title: f.title, Body: body, At: time.Now().Unix(),
		Seq: events.LatestSeq(), SeqRange: seqRange,
		Kind: f.kind, Tags: f.tags, Provenance: f.provenance,
	}
	if op.Kind == "" {
		op.Kind = kindForTopic(f.topic) // a stated topic implies the kind; --kind overrides
	}
	if f.hypothesis {
		op.Status = StatusHypothesis
	}
	if _, exists := findLive(live, op.ID); exists {
		return fmt.Errorf("id %s is a live entry — `gtmux knowledge supersede %s --title …` to replace it, or retitle", op.ID, op.ID)
	}
	auditNote := "add " + op.ID
	if len(f.captures) > 0 {
		consumed, err := consumeCandidateKeys(f.captures)
		if err != nil {
			return err
		}
		op.Capture = strings.Join(f.captures, ",")
		for _, c := range consumed {
			if c.Seq > 0 {
				op.Seqs = append(op.Seqs, c.Seq)
			}
		}
		newest := consumed[len(consumed)-1]
		op.Pane, op.Task = newest.Pane, newest.Task
		// Provenance from what was consumed: a mined lead is `mined`, a worker's capture
		// is `capture`; Hits counts every observation the candidates carried.
		mined := false
		for _, c := range consumed {
			if c.Source == "transcript" {
				mined = true
			}
			if c.Count > 1 {
				op.Hits += c.Count
			} else {
				op.Hits++
			}
		}
		if op.Provenance == "" {
			op.Provenance = ProvCapture
			if mined {
				op.Provenance = ProvMined
			}
		}
		auditNote += fmt.Sprintf(" (capture %s ×%d)", op.Capture, len(consumed))
	}
	if op.Provenance == "" {
		op.Provenance = ProvSelf
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
	supersedeID, err := entryID(pred.Topic, f.title)
	if err != nil {
		return err
	}
	op := knowledgeOp{
		Op: knowledgeOpSupersede, Supersedes: predID,
		ID: supersedeID, Topic: pred.Topic,
		Title: f.title, Body: body, At: time.Now().Unix(),
		Seq: events.LatestSeq(), Why: f.why,
		Kind: f.kind, Tags: f.tags, Provenance: f.provenance,
	}
	if f.hypothesis {
		op.Status = StatusHypothesis
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
	// The verb itself lives in knowledgeapi.go, which serve calls too. This function is
	// argument parsing; there is one implementation of what retiring an entry MEANS.
	return KnowledgeRetire(f.positional[0], f.why)
}

// knowledgeTopic DECLARES a custom topic (hq-open-topics): the vocabulary is the
// ledger's to extend, judged by the same validation capture uses.
func knowledgeTopic(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.why != "" {
		return fmt.Errorf("topic needs <name> and --desc (what belongs here)")
	}
	name := f.positional[0]
	desc := f.target // parsed below via --desc alias
	if f.descText != "" {
		desc = f.descText
	}
	if desc == "" {
		return fmt.Errorf("topic needs --desc — the description becomes the rendered file's intro")
	}
	_, custom, err := readKnowledgeState()
	if err != nil {
		return err
	}
	if err := validateTopicName(name, custom); err != nil {
		return err
	}
	if err := validateKnowledgeContent(desc, "", ""); err != nil {
		return err
	}
	op := knowledgeOp{
		Op: knowledgeOpTopic, ID: name, Topic: name, Title: desc,
		At: time.Now().Unix(), Seq: events.LatestSeq(),
	}
	return commitKnowledgeOp(op, "topic "+name)
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
		return fmt.Errorf("%s is already promoted and pending — land it (`gtmux knowledge land %s`) or `withdraw` it before promoting again", id, id)
	}
	// The audience replaces the free-text target (D4): "who must know" is a choice from
	// four, and each has an exit a person can actually take. A free-text target is
	// refused; a missing --for is tolerated (the screens gain their picker in phase 5)
	// but the brief says so, loudly.
	if f.target != "" {
		return fmt.Errorf("--target is gone: say who must know it with --for <%s|repo:<path>> (the brief carries the exit for each)",
			strings.Join([]string{AudienceHQ, AudienceMachine, AudienceEveryone}, "|"))
	}
	if f.audience == "" {
		i18n.Sae("⚠ no --for: who must know this? (hq | machine | repo:<path> | everyone) — the brief will have no exit until you `withdraw` and promote again with --for",
			"⚠ 没给 --for：这条给谁看？(hq | machine | repo:<路径> | everyone) —— 不选就没有出口，之后得 `withdraw` 再带 --for 重新晋升")
	}
	op := knowledgeOp{
		Op: knowledgeOpPromote, ID: id, Topic: entry.Topic,
		At: time.Now().Unix(), Seq: events.LatestSeq(),
		Why: f.why, Audience: f.audience, AudienceRepo: f.audienceRepo,
	}
	note := "promote " + id
	if f.audience != "" {
		note += " --for " + f.audience
		if f.audienceRepo != "" {
			note += ":" + f.audienceRepo
		}
	}
	return commitKnowledgeOp(op, note)
}

// knowledgeLand closes a pending promotion with the repo reference; the commit
// path's promotion sweep removes the brief.
// knowledgeLand closes a promotion. With --ref, the person carried it and says where.
// Without, gtmux carries it for the audiences it can reach (hq → LOCAL.md, machine → the
// canonical file and every agent's block, repo → that repository's instruction file,
// uncommitted) and the ref is where it wrote. `everyone` always needs the ref: the issue
// a person opened.
func knowledgeLand(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 {
		return fmt.Errorf("land needs <id> [--ref <where it landed>]")
	}
	id := f.positional[0]
	if f.ref != "" {
		return KnowledgeLand(id, f.ref)
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	entry, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	if !promotionPending(entry) {
		return fmt.Errorf("%s is not pending — nothing to land", id)
	}
	switch entry.Audience {
	case AudienceHQ:
		path, err := carryIntoLocal(entry)
		if err != nil {
			return err
		}
		i18n.Say("✓ written into "+path, "✓ 已写进 "+path)
		return KnowledgeLand(id, "LOCAL.md")
	case AudienceMachine:
		rep, err := SyncMachine(f.force)
		if err != nil {
			return err
		}
		sayRefused(rep)
		i18n.Say(fmt.Sprintf("✓ %s rendered · blocks written: %s · kept: %s", MachinePath(), strings.Join(rep.Written, ","), strings.Join(rep.Kept, ",")),
			fmt.Sprintf("✓ 已渲染 %s · 写入: %s · 已一致: %s", MachinePath(), strings.Join(rep.Written, ","), strings.Join(rep.Kept, ",")))
		return KnowledgeLand(id, MachinePath())
	case AudienceRepo:
		path, refused, err := SyncRepo(entry.AudienceRepo, f.force)
		if err != nil {
			return err
		}
		if refused {
			return fmt.Errorf("%s: gtmux's block was hand-edited — review it, then `land --force`", path)
		}
		i18n.Say("✓ written into "+path+" — NOT committed; that is yours", "✓ 已写进 "+path+"，未提交，提交由你来")
		return KnowledgeLand(id, path)
	case AudienceEveryone:
		return fmt.Errorf("everyone: open the issue first (%s), then `land %s --ref <issue url>`", IssueURL(entry), id)
	default:
		return fmt.Errorf("%s has no audience — `withdraw %s` then `promote %s --why … --for <hq|machine|repo:<path>|everyone>`, or land it yourself with --ref", id, id, id)
	}
}

func sayRefused(rep SyncReport) {
	if len(rep.Refused) > 0 {
		i18n.Sae("⚠ hand-edited, left alone (use --force to overwrite): "+strings.Join(rep.Refused, ", "),
			"⚠ 被手改过，没有动（--force 可覆盖）: "+strings.Join(rep.Refused, ", "))
	}
}

// knowledgeWithdraw returns a promoted entry to live: `gtmux knowledge withdraw <id> --why`.
func knowledgeWithdraw(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 || f.why == "" {
		return fmt.Errorf("withdraw needs <id> and --why (why this is not worth carrying — it survives in the journal)")
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
		return fmt.Errorf("%s is not pending — nothing to withdraw", id)
	}
	if err := validateKnowledgeContent("", "", f.why); err != nil {
		return err
	}
	op := knowledgeOp{Op: knowledgeOpWithdraw, ID: id, At: time.Now().Unix(), Seq: events.LatestSeq(), Why: f.why}
	return commitKnowledgeOp(op, "withdraw "+id+": "+f.why)
}

// knowledgeSync refreshes every carrier: `gtmux knowledge sync [--force] [--repo <path>]`.
// It runs from anywhere — the carriers are the user's files, not the supervisor's.
func knowledgeSync(args []string) int {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		i18n.Sae("gtmux knowledge sync: "+err.Error(), "gtmux knowledge sync: "+err.Error())
		return 2
	}
	if f.repo != "" {
		path, refused, err := SyncRepo(f.repo, f.force)
		if err != nil {
			i18n.Sae("gtmux knowledge sync: "+err.Error(), "gtmux knowledge sync: "+err.Error())
			return 1
		}
		if refused {
			i18n.Sae("⚠ "+path+": hand-edited, left alone (--force to overwrite)", "⚠ "+path+"：被手改过，没有动（--force 可覆盖）")
			return 1
		}
		i18n.Say("✓ "+path+" — not committed", "✓ "+path+"，未提交")
		return 0
	}
	rep, err := SyncMachine(f.force)
	if err != nil {
		i18n.Sae("gtmux knowledge sync: "+err.Error(), "gtmux knowledge sync: "+err.Error())
		return 1
	}
	if f.jsonOut {
		b, _ := json.Marshal(rep)
		fmt.Println(string(b))
		return 0
	}
	sayRefused(rep)
	i18n.Say(fmt.Sprintf("✓ %s · written: %s · in sync: %s", MachinePath(), orNone(rep.Written), orNone(rep.Kept)),
		fmt.Sprintf("✓ %s · 写入: %s · 已一致: %s", MachinePath(), orNone(rep.Written), orNone(rep.Kept)))
	if len(rep.Refused) > 0 {
		return 1
	}
	return 0
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "—"
	}
	return strings.Join(s, ",")
}

// knowledgeCarriers lists the carriers and their state: `gtmux knowledge carriers [--json]`.
func knowledgeCarriers(args []string) int {
	f, _ := parseKnowledgeFlags(args)
	sts, err := CarrierStatuses()
	if err != nil {
		i18n.Sae("gtmux knowledge carriers: "+err.Error(), "gtmux knowledge carriers: "+err.Error())
		return 1
	}
	if f.jsonOut {
		if sts == nil {
			sts = []CarrierStatus{}
		}
		b, _ := json.Marshal(sts)
		fmt.Println(string(b))
		return 0
	}
	for _, s := range sts {
		fmt.Printf("  %-10s %-13s %s\n", s.Agent, s.State, s.Path)
	}
	return 0
}

// parseAudience reads a --for value: hq | machine | everyone | repo:<path>.
func parseAudience(v string) (audience, repo string, err error) {
	if strings.HasPrefix(v, AudienceRepo+":") {
		p := strings.TrimPrefix(v, AudienceRepo+":")
		if p == "" {
			return "", "", fmt.Errorf("--for repo:<path> needs the repository path")
		}
		return AudienceRepo, cleanRepo(p), nil
	}
	if v == AudienceRepo {
		return "", "", fmt.Errorf("--for repo needs the path: --for repo:<path>")
	}
	if !validAudience(v) {
		return "", "", fmt.Errorf("unknown audience %q (want hq | machine | repo:<path> | everyone)", v)
	}
	return v, "", nil
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
		len(pending), humanize.AgeShort(now-oldestAt)),
		fmt.Sprintf("%d 条待落地晋升,最久 %s 前:", len(pending), humanize.AgeShort(now-oldestAt)))
	for _, op := range pending {
		aud := op.Audience
		if aud == "" {
			aud = "?"
		}
		if op.AudienceRepo != "" {
			aud += ":" + op.AudienceRepo
		}
		fmt.Printf("  %-40s  %-4s  for %-9s  (%s)\n", op.ID, humanize.AgeShort(now-op.PromotedAt), aud, promotionBriefPath(op))
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
	if len(f.captures) == 0 || f.why == "" {
		return fmt.Errorf("dismiss needs --capture <key> and --why")
	}
	if err := validateKnowledgeContent("", "", f.why); err != nil {
		return err
	}
	consumed, err := consumeCandidateKeys(f.captures)
	if err != nil {
		return err
	}
	keys := strings.Join(f.captures, ",")
	events.AuditKnowledge(fmt.Sprintf("dismiss %s ×%d: %s", keys, len(consumed), f.why),
		time.Now().Unix())
	i18n.Say(fmt.Sprintf("dismissed %d candidate(s) under %s", len(consumed), keys),
		fmt.Sprintf("已驳回 %d 条候选（%s）", len(consumed), keys))
	return nil
}

// commitKnowledgeOp is the shared mutation tail: append, re-render, journal.
func commitKnowledgeOp(op knowledgeOp, auditNote string) error {
	if err := appendKnowledgeOp(op); err != nil {
		return err
	}
	live, custom, err := readKnowledgeState()
	if err != nil {
		return err
	}
	if err := renderAllTopics(live, custom, time.Now().Unix()); err != nil {
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
	live, custom, err := readKnowledgeState()
	if err != nil {
		return err
	}
	if check {
		drifted := knowledgeDrift(live, custom)
		if len(drifted) == 0 {
			return nil
		}
		for _, p := range drifted {
			i18n.Sae("drift: "+p+" no longer matches its render — hand edits go through `gtmux knowledge`; `gtmux knowledge render` to restore",
				"drift: "+p+" 与生成结果不一致 —— 手改请走 `gtmux knowledge`；`gtmux knowledge render` 可恢复")
		}
		return fmt.Errorf("%d rendered file(s) drifted", len(drifted))
	}
	return renderAllTopics(live, custom, time.Now().Unix())
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
		if (f.topic == "" || op.Topic == f.topic) && (f.kind == "" || op.Kind == f.kind) {
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
  add       --topic <t> --title "<one line>" [--body-file <path|->] [--capture <key>[,<key>…]] [--seq-range a..b]
  supersede <id> --title "<one line>" [--body-file <path|->] [--why "<reason>"]
  retire    <id> --why "<reason>"
  dismiss   --capture <key>[,<key>…] --why "<reason>"
  topic     <name> --desc "<what belongs here>"            # declare your own topic
            add/supersede also take --kind <facts|howto|pitfalls|judgment|decisions>
            [--tags a,b] [--provenance <correction|recurrence|mined|capture|self>] [--hypothesis]
  kind      <id> <kind>                                    # confirm or correct an entry's kind
  hit       <id> [--n N] [--why "<where>"]                 # the lesson was hit again
  confirm   <id>                                           # a hypothesis held up
  promote   <id> --why "<case>" --for <hq|machine|repo:<path>|everyone>   # who must know it
  land      <id> [--ref "<where>"] [--force]   # no --ref: gtmux writes it (LOCAL.md / machine blocks / repo file)
  withdraw  <id> --why "<reason>"              # the entry was right, the promotion was not
  sync      [--force] [--repo <path>] [--json] # refresh every agent's knowledge block (or one repo's)
  carriers  [--json]                           # each agent's instruction file and whether it is in sync
  promotions [--json]                                      # the pending export queue
  mine      [--dry-run] [--since <Nd>|all] [--status]      # mine session logs into the spool
  list      [--topic <t>] [--kind <k>] [--json]     show <id>     render [--check]
  The knowledge base's authority is an append-only ledger; topic .md files are
  rendered from it, entries carry provenance (seq/pane/task/capture), and every
  mutation is journaled. A charter-level lesson exits through promote → a brief
  under knowledge/promotions/ → land. Mutations run from the HQ home only;
  workers use `+"`gtmux capture`"+`.

  THIS LEDGER IS ONE OF THREE places a rule can live: gtmux's shipped charter
  (AGENTS.md, reaches every machine through a version bump), the operator's own
  LOCAL.md (never overwritten, in context every turn), and this ledger — this
  machine's, spent at dispatch rather than always present. A lesson filed in the
  wrong one is a lesson its readers never get. docs/design/knowledge-layers.md.`,
		`用法：gtmux knowledge <子命令>
  add       --topic <主题> --title "<一句话>" [--body-file <路径|->] [--capture <键>[,<键>…]] [--seq-range a..b]
  supersede <id> --title "<一句话>" [--body-file <路径|->] [--why "<原因>"]
  retire    <id> --why "<原因>"
  dismiss   --capture <键>[,<键>…] --why "<原因>"
  topic     <名称> --desc "<这里放什么>"               # 声明你自己的主题
            add/supersede 还接受 --kind <facts|howto|pitfalls|judgment|decisions>
            [--tags a,b] [--provenance <correction|recurrence|mined|capture|self>] [--hypothesis]
  kind      <id> <种类>                               # 确认或改正一条的种类
  hit       <id> [--n N] [--why "<在哪>"]             # 这条教训又被踩到了
  confirm   <id>                                      # 猜想被证实，转正
  promote   <id> --why "<理由>" --for <hq|machine|repo:<路径>|everyone>   # 这条给谁看
  land      <id> [--ref "<落在哪>"] [--force]   # 不带 --ref 就由 gtmux 写进去（LOCAL.md / 各 agent 的块 / 仓库文件）
  withdraw  <id> --why "<原因>"                 # 条目没错，只是不值得搬
  sync      [--force] [--repo <路径>] [--json]  # 刷新每个 agent 的知识块（或某个仓库的）
  carriers  [--json]                            # 各 agent 的指令文件与是否同步
  promotions [--json]                                 # 待落地队列
  mine      [--dry-run] [--since <N>d|all] [--status] # 从会话日志采矿进待蒸馏队列
  list      [--topic <主题>] [--kind <种类>] [--json]     show <id>     render [--check]
  知识库以追加式台账为准，主题 .md 由它生成；条目携带来源证据（seq/pane/task/capture），
  每次变更都写入事件流。守则级教训经 promote 生成 knowledge/promotions/ 下的简报,
  落地后用 land 闭环。变更只能在中控目录执行；worker 用 `+"`gtmux capture`"+`。

  本台账是「规矩能存放的三处」之一:gtmux 出厂章程(AGENTS.md,靠升版本号下发到每台
  机器)、你自己的 LOCAL.md(永不覆盖,每轮都在上下文里),以及本台账 —— 这台机器的,
  派活时才浮出来而不是常驻。放错一层,等于写给不会读到它的人看。
  详见 docs/design/knowledge-layers.md。`)
	return 0
}

// entryID builds an entry's id from its topic and title, refusing a title that carries no
// ASCII word to slug.
//
// The old fallback was the string "untagged", which failed twice over: every
// Chinese-titled entry landed on the SAME id, and once that id was live the next one
// failed with `id <topic>/untagged is a live entry` — an error about a collision, which
// sends the reader looking for a duplicate that does not exist. Four reproductions on this
// machine, each one a detour.
//
// The convention that works is already in the base (`hq-send-destroys-drafts`,
// `edit-markdown-table-old-string-new`): an ascii slug for the machine, a sentence for the
// human. So say that, at the moment it is needed, instead of inventing an id nobody asked
// for.
func entryID(topic, title string) (string, error) {
	sl := Slug(title)
	if sl == untaggedSlug {
		return "", fmt.Errorf(i18n.Tr(
			"a title with no ASCII word cannot become an id — write it as `<ascii-slug>: %s`",
			"标题里没有 ASCII 词，生成不出 id —— 请写成 `<ascii-slug>: %s`"), title)
	}
	return topic + "/" + sl, nil
}

// splitKeys reads one --capture value: a key, or several joined by commas.
func splitKeys(v string) []string {
	var out []string
	for _, k := range strings.Split(v, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// consumeCandidateKeys consumes every key, all or nothing: an unknown key fails the
// whole call BEFORE any spool line is removed, so a typo in the third key cannot leave
// the first two consumed and the entry unwritten.
func consumeCandidateKeys(keys []string) ([]Candidate, error) {
	cands, err := readCandidates()
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, c := range cands {
		have[c.Key] = true
	}
	for _, k := range keys {
		if !have[k] {
			return nil, fmt.Errorf("no pending candidate with key %q (gtmux capture --list)", k)
		}
	}
	var all []Candidate
	for _, k := range keys {
		consumed, err := consumeCandidates(k)
		if err != nil {
			return nil, err
		}
		all = append(all, consumed...)
	}
	return all, nil
}

// knowledgeKind confirms or corrects a live entry's kind: `gtmux knowledge kind <id> <kind>`.
func knowledgeKind(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 2 {
		return fmt.Errorf("kind needs <id> <kind> (%s)", strings.Join(Kinds, " | "))
	}
	id, kind := f.positional[0], f.positional[1]
	if !validKind(kind) {
		return fmt.Errorf("unknown kind %q (want %s)", kind, strings.Join(Kinds, " | "))
	}
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	if _, ok := findLive(live, id); !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	op := knowledgeOp{Op: knowledgeOpKind, ID: id, Kind: kind, At: time.Now().Unix(), Seq: events.LatestSeq()}
	return commitKnowledgeOp(op, "kind "+id+" = "+kind)
}

// knowledgeHit records a filed lesson being hit again: `gtmux knowledge hit <id> [--n N]
// [--why "<where>"]`. This is the recurrence counter on the entry — the signal that the
// carrier failed, not the memory.
func knowledgeHit(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 {
		return fmt.Errorf("hit needs <id>")
	}
	n := f.n
	if n <= 0 {
		n = 1
	}
	id := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	if _, ok := findLive(live, id); !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	op := knowledgeOp{Op: knowledgeOpHit, ID: id, Hits: n, At: time.Now().Unix(), Seq: events.LatestSeq(), Why: f.why}
	return commitKnowledgeOp(op, fmt.Sprintf("hit %s ×%d", id, n))
}

// knowledgeConfirm turns a hypothesis into a live entry: `gtmux knowledge confirm <id>`.
func knowledgeConfirm(args []string) error {
	f, err := parseKnowledgeFlags(args)
	if err != nil {
		return err
	}
	if len(f.positional) != 1 {
		return fmt.Errorf("confirm needs <id>")
	}
	id := f.positional[0]
	live, err := liveKnowledge()
	if err != nil {
		return err
	}
	e, ok := findLive(live, id)
	if !ok {
		return fmt.Errorf("no live entry %q (gtmux knowledge list)", id)
	}
	if e.Status != StatusHypothesis {
		return fmt.Errorf("%s is not a hypothesis", id)
	}
	op := knowledgeOp{Op: knowledgeOpConfirm, ID: id, At: time.Now().Unix(), Seq: events.LatestSeq()}
	return commitKnowledgeOp(op, "confirm "+id)
}
