package knowledge

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// The writing rules for a knowledge entry, as one table (kb-plain-language-rules).
//
// They used to live twice: a paragraph in HQ's charter, which HQ reads before it writes,
// and a set of regular expressions in voice.go, which the lint runs afterwards. The two
// never agreed on what they were asking for, and editing one left the other where it was.
// This table is the single copy. The lint takes its matchers from here, `gtmux knowledge
// style` prints it, and the charter points at the command.
//
// Two things about the shape:
//
//   - A rule says how confidently it can be JUDGED. A mechanical rule carries a matcher
//     and the lint runs it; a judgement rule carries none, and no check pretends to settle
//     it. The tiering is the humanizer skill's: a strong tell counts on one sighting, a
//     weak one only alongside another, because a dash or a bold run has honest uses and
//     two kinds together are the shape of prose written by rule.
//   - A rule carries a BEFORE and an AFTER. The 2026-09-16 pass rewrote 485 of 505 entries
//     by hand, and what made it land was the pair of sentences. A rule stated without one
//     is a rule nobody can apply to the sentence in front of them.
//
// The reader an entry is written for is two readers: a person who wants to understand it
// and an agent that wants to comply with it. Rules 15 and 16 are here for the second one,
// and the humanizer skill has no reason to carry either.

// Text is a rule's two halves, each written for its own language's reader.
type Text struct {
	En string `json:"en"`
	Zh string `json:"zh"`
}

// Rule tiers. Strong and weak are mechanical; judgement has no matcher.
const (
	TierStrong = "strong"
	TierWeak   = "weak"
	TierJudge  = "judgement"
)

// Rule is one writing rule: what to do, why, and the pair of sentences that shows it.
type Rule struct {
	ID     string `json:"id"`
	Tier   string `json:"tier"`
	What   Text   `json:"what"`
	Why    Text   `json:"why"`
	Before Text   `json:"before"`
	After  Text   `json:"after"`
	// find reports the finding's detail for an entry, or "" when the rule is kept. A
	// judgement rule leaves it nil: only a reader settles those.
	find func(entryProse) string
}

// Mechanical reports whether the lint can judge this rule.
func (r Rule) Mechanical() bool { return r.find != nil }

// entryProse is what a matcher reads: an entry's parts with code, quoted spans, indented
// blocks and table rows already stripped from the prose. ID and TitleRaw keep their
// originals, because two of the rules are about the title's own shape.
type entryProse struct {
	ID       string
	Title    string
	TitleRaw string
	Body     string
	All      string
	// Hypothesis exempts an entry from the guess rule: --hypothesis is the honest way to
	// shelve a lead, and the rule is about a lead filed as a finding.
	Hypothesis bool
}

// proseOf builds the reading for one live entry, its other-language half included.
func proseOf(op knowledgeOp) entryProse {
	return entryProse{
		ID:         op.ID,
		Title:      voiceProse(op.Title),
		TitleRaw:   op.Title,
		Body:       voiceProse(op.Body),
		All:        voiceProse(op.Title + "\n" + op.Body + "\n" + altText(op)),
		Hypothesis: op.Status == StatusHypothesis,
	}
}

// Matchers for the rules that voice.go did not already carry.
var (
	// A guess dressed as a finding. Deliberately narrow: "可能导致" is a real claim about
	// a risk, and only the shapes that stand in for evidence are here. A hypothesis entry
	// is exempt — `--hypothesis` is the honest way to shelve a lead, and this rule exists
	// to stop a lead from being filed as a fact, not to discourage filing one.
	guessRe = regexp.MustCompile(`(?i)\b(?:presumably|it is believed that|based on (?:the )?available information|not (?:publicly|widely) (?:available|documented)|as of my (?:last )?(?:training|knowledge))\b|我猜|大概率是|估计(?:是|有|会)|不出意外的话|推测(?:是|应该|为)|应该是[^。；\n]{0,20}吧`)
	// An identifier used where the sentence needed the thing it names. Only what is NOT in
	// backticks reaches this: an author who marked it as code meant it as code.
	identRe = regexp.MustCompile(`\b_?[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b|\b[a-z]+[A-Z][A-Za-z0-9]*\b`)
	// Product names that are spelled this way by their owners, not by an author reaching
	// for a field name.
	identOK = map[string]bool{
		"macos": true, "ios": true, "ipados": true, "iphone": true, "ipad": true,
		"watchos": true, "tvos": true, "javascript": true, "typescript": true,
		"github": true, "gitlab": true, "openspec": true, "jsonl": true,
	}
)

// titleEchoFloor is how much of the title may reappear in the body's first sentence before
// the sentence is judged to be restating it.
const titleEchoFloor = 0.7

func findGuess(e entryProse) string {
	if e.Hypothesis {
		return ""
	}
	if m := guessRe.FindString(e.All); m != "" {
		return "a guess stated as a fact (" + strings.TrimSpace(m) + ") — say what is known, or file it with --hypothesis"
	}
	return ""
}

func findTitleEcho(e entryProse) string {
	head := firstLine(e.Body)
	if head == "" || e.Title == "" {
		return ""
	}
	t, h := tokens(e.Title), tokens(head)
	if len(t) == 0 {
		return ""
	}
	if overlap(t, h) >= titleEchoFloor {
		return "the body opens by restating the title — start with what the reader does not have yet"
	}
	return ""
}

func findIdentInTitle(e entryProse) string {
	var hits []string
	seen := map[string]bool{}
	for _, m := range identRe.FindAllString(e.Title, -1) {
		if identOK[strings.ToLower(m)] || seen[m] {
			continue
		}
		seen[m] = true
		hits = append(hits, m)
	}
	if len(hits) == 0 {
		return ""
	}
	return "the title names the implementation (" + strings.Join(hits, ", ") + ") — say what it does, and keep the name in the body where it can be explained"
}

// checkVoice and checkTitle are the lint checks a rule's findings are reported under. A
// voice finding is a sentence to rewrite; a title finding is a title to rewrite, which is
// a different act on a different line, so it gets its own name.
const (
	checkVoice = "ai-voice"
	checkTitle = "title-unreadable"
)

// Check is the lint check this rule reports under.
func (r Rule) Check() string {
	switch r.ID {
	case "implementation-name-as-title":
		return checkTitle
	default:
		return checkVoice
	}
}

// rules is the table. Order is the order `gtmux knowledge style` prints, and the order the
// numbers come from: the mechanical rules first, strongest first, then the judgements.
var rules = []Rule{
	{
		ID:   "chat-residue",
		Tier: TierStrong,
		What: Text{
			En: "Cut the greeting, the praise and the offer to help. An entry stands on its own.",
			Zh: "问候、夸奖、「还需要我做什么吗」，一概删掉。条目是独立的一段话，不是聊天记录里截下来的一段。",
		},
		Why: Text{
			En: "It is a chat wrapper around real content, and the surest single sign that a machine wrote the line rather than thought it.",
			Zh: "那是包在内容外面的聊天壳子。它也是「这句是机器写的」最确定的一个信号。",
		},
		Before: Text{
			En: "Great question! Here is what goes wrong with the tunnel. Let me know if you want more detail.",
			Zh: "好问题！隧道这里的毛病是这样的。还需要我展开吗？",
		},
		After: Text{
			En: "The tunnel drops when the office proxy resets TLS.",
			Zh: "办公室的代理会重置 TLS，隧道因此断开。",
		},
		find: func(e entryProse) string {
			if voiceResidueRe.MatchString(e.All) {
				return "chat residue — an entry is not a reply; cut the wrapper and keep the content"
			}
			return ""
		},
	},
	{
		ID:   "decorative-arrow",
		Tier: TierStrong,
		What: Text{
			En: "Write the sentence. An ⇒ between two nouns is not a verb. A → in a real sequence of steps is fine.",
			Zh: "把句子写出来。两个名词中间放一个 ⇒ 并不等于说清了什么。真的是一串步骤时，用 → 没问题。",
		},
		Why: Text{
			En: "The arrow hides which way the relation runs and what the reader is supposed to do about it.",
			Zh: "箭头把关系藏起来了：谁导致谁、读者该做什么，一样都没说。",
		},
		Before: Text{
			En: "corp proxy ⇒ TLS reset ⇒ retry",
			Zh: "公司代理 ⇒ TLS 重置 ⇒ 重试",
		},
		After: Text{
			En: "The corporate proxy resets TLS. Retry once; the second attempt usually goes through.",
			Zh: "公司代理会重置 TLS。重试一次，第二次一般就过了。",
		},
		find: func(e entryProse) string {
			if n := len(voiceArrowRe.FindAllString(e.All, -1)); n > 0 {
				return fmt.Sprintf("⇒ ×%d (write the sentence)", n)
			}
			return ""
		},
	},
	{
		ID:   "house-jargon",
		Tier: TierStrong,
		What: Text{
			En: "A word only this machine uses is replaced by the plain one. A term with a real name is explained once, then used.",
			Zh: "只有本机懂的自造词，换成通用的说法。真有正式名字的术语，第一次出现时给一句解释，之后照用。",
		},
		Why: Text{
			En: "The reader is whoever opens this in three months, usually another agent, and a private coinage costs them a lookup they cannot make.",
			Zh: "读者是三个月后第一次打开它的人，多半是另一个 agent。自造词让他去查一个查不到的东西。",
		},
		Before: Text{
			En: "Re-verify the vessel, then read its same-kin entries.",
			Zh: "派活之前先反验这条船。",
		},
		After: Text{
			En: "Check that the session is still alive, then read the entries next to it.",
			Zh: "派活之前，先确认那个会话还活着。",
		},
		find: func(e entryProse) string {
			var out []string
			for _, j := range voiceJargon {
				if n := strings.Count(e.All, j.word); n > 0 {
					out = append(out, fmt.Sprintf("%s ×%d (say %s)", j.word, n, j.say))
				}
			}
			return strings.Join(out, " · ")
		},
	},
	{
		ID:   "guess-as-fact",
		Tier: TierStrong,
		What: Text{
			En: "Do not file a guess as a finding. Say what is known, say what is not, and shelve an unverified lead with --hypothesis.",
			Zh: "别把猜测当结论写进来。知道的说知道，不知道的说不知道；没验证的线索用 --hypothesis 挂起来。",
		},
		Why: Text{
			En: "An agent reading the base acts on it. A sentence that sounds settled and is not sends the next session down a road nobody checked.",
			Zh: "agent 读了知识库就会照做。一句听上去已成定论、其实没验过的话，会把下一个会话领到没人走过的路上。",
		},
		Before: Text{
			En: "The push presumably fails because of the relay's token cache.",
			Zh: "推送失败估计是 relay 那边 token 缓存的问题。",
		},
		After: Text{
			En: "The push fails and the cause is not established. The relay's token cache is the first thing to rule out.",
			Zh: "推送会失败，原因还没查清。第一个要排除的是 relay 的 token 缓存。",
		},
		find: findGuess,
	},
	{
		ID:   "title-echoed-in-body",
		Tier: TierStrong,
		What: Text{
			En: "The body starts with what the title did not have room for, never with the title again.",
			Zh: "正文第一句写标题装不下的那部分，不要把标题再说一遍。",
		},
		Why: Text{
			En: "In every render the two sit next to each other, so a restatement costs the reader the one line that was going to tell them something.",
			Zh: "每一处渲染里这两行都是挨着的。重复一遍，等于把唯一一行能给新信息的位置浪费掉。",
		},
		Before: Text{
			En: "Title: restore injects agents into panes that never ran one\nBody: restore injects agents into panes that never ran one. It happens because…",
			Zh: "标题：restore 会往没跑过 agent 的 pane 里注入 agent\n正文：restore 会往没跑过 agent 的 pane 里注入 agent。原因是……",
		},
		After: Text{
			En: "Title: restore injects agents into panes that never ran one\nBody: The save records a pane's last command, and a pane that only ever held a shell still has one.",
			Zh: "标题：restore 会往没跑过 agent 的 pane 里注入 agent\n正文：存档记的是 pane 最后跑过的命令，而只开过 shell 的 pane 也有这么一条。",
		},
		find: findTitleEcho,
	},
	{
		ID:   "implementation-name-as-title",
		Tier: TierStrong,
		What: Text{
			En: "The title says what happens. A field, a flag or a status code goes in the body, where there is room to explain it.",
			Zh: "标题说发生了什么。字段名、状态码、判据名放到正文里，那里有地方解释它。",
		},
		Why: Text{
			En: "The reader who most needs the title is the one who does not know this code yet. A name they cannot resolve is a line they skip.",
			Zh: "最需要这个标题的，恰恰是还不熟这块代码的人。一个他解析不了的名字，就是一行他会跳过的字。",
		},
		Before: Text{
			En: "sourceKey is empty when hqNudge is false",
			Zh: "hqNudge 为 false 时 sourceKey 为空",
		},
		After: Text{
			En: "A wake with nudging turned off records no origin, so the event cannot be traced back",
			Zh: "关掉唤醒之后，事件不再记来源，事后追不回是谁触发的",
		},
		find: findIdentInTitle,
	},
	{
		ID:   "dash-chain",
		Tier: TierWeak,
		What: Text{
			En: "Do not string clauses on dashes. Use a period, a comma or a colon, or rewrite the sentence.",
			Zh: "不要用破折号把从句一节一节串下去。用句号、逗号或冒号，实在不行就把句子重写。",
		},
		Why: Text{
			En: "The dash lets the writer skip deciding how the two halves relate, so the reader has to decide instead.",
			Zh: "破折号让写的人省掉了「这两半是什么关系」这个决定，于是这个决定落到了读的人头上。",
		},
		Before: Text{
			En: "The move takes seconds — every paired device drops — and comes back by itself — usually.",
			Zh: "迁移只要几秒 —— 每台配对设备都会掉线 —— 然后自己回来 —— 一般是这样。",
		},
		After: Text{
			En: "The move takes a few seconds. Every paired device drops and reconnects by itself.",
			Zh: "迁移要几秒钟。每台配对设备都会掉线，然后自己连回来。",
		},
		find: func(e entryProse) string {
			if n := len(voiceDashRe.FindAllString(e.All, -1)); n >= 2 {
				return fmt.Sprintf("%d dashes", n)
			}
			return ""
		},
	},
	{
		ID:   "bold-as-decoration",
		Tier: TierWeak,
		What: Text{
			En: "Bold the one thing that would change a decision. A section with three bold labels has none.",
			Zh: "加粗留给会改变决定的那一处。一节里三处加粗，等于一处都没有。",
		},
		Why: Text{
			En: "Bold on every item is formatting applied by rule, and it flattens the one line that needed to stand out.",
			Zh: "每条都加粗，是按规则排版，不是按内容排版。真正该跳出来的那一行也就跟着淹没了。",
		},
		Before: Text{
			En: "**Symptom:** the pane freezes. **Cause:** a wedged ps. **Fix:** abandon the read.",
			Zh: "**症状：**面板卡住。**原因：**ps 卡死。**做法：**放弃这次读取。",
		},
		After: Text{
			En: "The pane freezes because a wedged ps never returns. Abandon the read rather than waiting on it.",
			Zh: "ps 卡死不返回，面板就跟着冻住。别等它，直接放弃这次读取。",
		},
		find: func(e entryProse) string {
			if n := len(voiceBoldRe.FindAllString(e.All, -1)); n >= 2 {
				return fmt.Sprintf("%d bold runs", n)
			}
			return ""
		},
	},
	{
		ID:   "staged-contrast",
		Tier: TierWeak,
		What: Text{
			En: "Write not X but Y only when the reader actually believes X. Otherwise say Y.",
			Zh: "只有读者真的以为是 X 的时候，才写「不是 X，是 Y」。否则直接说 Y。",
		},
		Why: Text{
			En: "The negative half names a belief nobody holds so the positive half sounds larger. It adds weight without adding a claim.",
			Zh: "前半句立一个没人持有的看法，好让后半句显得更有分量。分量加了，内容没加。",
		},
		Before: Text{
			En: "This is not only a tmux problem but a locale one.",
			Zh: "这不是 tmux 的问题，是 locale 的问题。",
		},
		After: Text{
			En: "The launchd serve inherits no LANG, so tmux mangles the agent's status glyph.",
			Zh: "launchd 起的 serve 没有 LANG，tmux 因此把 agent 的状态符号弄花了。",
		},
		find: func(e entryProse) string {
			if n := len(voiceNotXRe.FindAllString(e.All, -1)); n >= 1 {
				return fmt.Sprintf("%d “not X, Y”", n)
			}
			return ""
		},
	},
}

// judgements are the rules no matcher can settle. They are stated here, in the same shape
// and with the same examples, and the lint says nothing about them on purpose: whether a
// contrast is earned or a sentence carries a fact is reading, not matching.
var judgements = []Rule{
	{
		ID:   "slug-in-title",
		Tier: TierJudge,
		What: Text{
			En: "The title is a sentence, not the id repeated.",
			Zh: "标题是一句话，不是把 id 再抄一遍。",
		},
		Why: Text{
			En: "The index gives the title one line, and the id is already on the next one. A leading slug is stripped when the index is rendered, which is a patch for what is already written, not a licence for the next entry.",
			Zh: "索引里标题只有一行，id 就印在它下面一行。开头的 slug 在渲染时会被去掉，但那是给已经写下的条目兜底，不是给下一条的许可。",
		},
		Before: Text{
			En: "alias-makes-destructive-ops-silently-noop: interactive aliases make destructive commands do nothing",
			Zh: "alias-makes-destructive-ops-silently-noop 交互式别名让破坏性操作静默不执行",
		},
		After: Text{
			En: "Interactive aliases make rm and cp do nothing in a script",
			Zh: "交互式别名让脚本里的 rm 和 cp 什么也没干",
		},
	},
	{
		ID:   "lead-with-the-finding",
		Tier: TierJudge,
		What: Text{
			En: "Open with what happened or what to do. The evidence comes after it.",
			Zh: "开头就写发生了什么、该怎么做。证据放在后面。",
		},
		Why: Text{
			En: "The entry is consulted in the middle of something else. A reader who has to reach the third paragraph for the point usually does not.",
			Zh: "读这条的人正在干别的事，是中途来查的。要读到第三段才看到结论，他多半就不读了。",
		},
		Before: Text{
			En: "During the 2026-08-03 review of the wake path, several sessions were examined, and after comparing their transcripts it emerged that HQ had read its own output as the commander's.",
			Zh: "在 2026-08-03 对唤醒链路的复盘中，我们检查了若干会话，对比 transcript 之后发现，HQ 把自己的输出当成了司令的输入。",
		},
		After: Text{
			En: "HQ read its own output as the commander's and withdrew a correct suspicion. It happened on 2026-08-03, in a session near its context limit.",
			Zh: "HQ 把自己的输出当成司令的输入，于是撤回了一个本来正确的怀疑。发生在 2026-08-03，那个会话当时已经接近上下文上限。",
		},
	},
	{
		ID:   "one-idea-per-sentence",
		Tier: TierJudge,
		What: Text{
			En: "One idea per sentence, one thing per paragraph.",
			Zh: "一句话一个意思，一段话一件事。",
		},
		Why: Text{
			En: "Three clauses joined by commas make the reader hold all three before any of them resolves, and the one that mattered is usually the last.",
			Zh: "三个从句用逗号连起来，读者得把三件事同时记着，等到最后才知道哪件是重点。",
		},
		Before: Text{
			En: "The installer withdraws its own site when nginx refuses it, which it does because the config uses 1.25 syntax on a 1.24 box, so the live website never stops serving.",
			Zh: "安装脚本在 nginx 拒绝配置时会撤回自己那份站点配置，之所以被拒是因为配置用了 1.25 的语法而机器上是 1.24，所以线上网站始终没有中断。",
		},
		After: Text{
			En: "The config used 1.25 syntax on a box running 1.24, so nginx refused it. The installer withdrew its own site file, and the live website never stopped serving.",
			Zh: "机器上是 nginx 1.24，配置却用了 1.25 的语法，所以被拒了。安装脚本撤回了自己那份配置，线上网站一直正常。",
		},
	},
	{
		ID:   "explain-the-term",
		Tier: TierJudge,
		What: Text{
			En: "Say the plain thing first, then the term in brackets. A term with a formal name keeps it and gets one line of gloss.",
			Zh: "先说人话，再把术语放在后面的括号里。方法论里有正式名字的，名字保留，加一句解释。",
		},
		Why: Text{
			En: "A term is not banned, it is explained. The reader needs to match what they already know against what this entry calls it.",
			Zh: "术语不是禁用，是要解释。读者需要把自己已经懂的东西和这条里的叫法对上号。",
		},
		Before: Text{
			En: "The oracle is derived from the hunk, projected onto the diff.",
			Zh: "判据从 hunk 推导，投影到 diff 上。",
		},
		After: Text{
			En: "The check is built from the changed lines themselves, so there is a ready-made way to judge the change (an oracle).",
			Zh: "判断依据直接从改动的那几行里取，所以不用另找标准（这就是所谓的 oracle，现成的判断依据）。",
		},
	},
	{
		ID:   "verbatim-stays-verbatim",
		Tier: TierJudge,
		What: Text{
			En: "Everything executable or checkable stays word for word: commands, paths, thresholds, ids, dates, numbers, error text, and the commander's own words.",
			Zh: "能执行、能核对的东西逐字不动：命令、路径、阈值、id、日期、数字、报错原文，以及司令自己说的话。",
		},
		Why: Text{
			En: "The rewrite is of the saying, never of the fact. An entry is read by an agent that will run what it finds, and a prettier command is a wrong command.",
			Zh: "改的是说法，不是事实。读这条的 agent 会照着跑，一个被改漂亮的命令就是一个错的命令。",
		},
		Before: Text{
			En: "Remove the file forcibly, bypassing the interactive alias.",
			Zh: "用绕过交互别名的方式强制删掉那个文件。",
		},
		After: Text{
			En: "Run `command rm -f <path>`: the plain `rm` here is aliased to `rm -i` and waits for a confirmation that never comes.",
			Zh: "跑 `command rm -f <路径>`：本机的 `rm` 被别名成了 `rm -i`，会一直等一个永远不会来的确认。",
		},
	},
	{
		ID:   "who-where-what-todo",
		Tier: TierJudge,
		What: Text{
			En: "An entry says who, where, what happened, and what to do instead.",
			Zh: "一条条目要说清：谁、在哪、发生了什么、该怎么改。",
		},
		Why: Text{
			En: "An entry that can be understood but not acted on is not worth the line it takes in every agent's instruction file.",
			Zh: "读得懂但没法照做的条目，不值得占据每个 agent 指令文件里的那一行。",
		},
		Before: Text{
			En: "Verification was insufficient, so the change did not hold.",
			Zh: "验证不充分，改动没有站住。",
		},
		After: Text{
			En: "A gate was judged by its last printed line instead of its exit code, so a failing check read as green. Judge every gate by `$?`.",
			Zh: "有人看门禁的最后一行输出，没看退出码，于是一个失败的检查被当成了通过。每个门禁都用 `$?` 判。",
		},
	},
	{
		ID:   "show-both-when-choosing",
		Tier: TierJudge,
		What: Text{
			En: "When the entry exists to help someone choose, show both options and what each costs. Do not argue for one.",
			Zh: "如果这条是给人做选择用的，把两个选项和各自的代价都摆出来，不要替他论证其中一个。",
		},
		Why: Text{
			En: "The reader's situation is not the one the entry was written in, and the trade-off is what transfers; the verdict is not.",
			Zh: "读者的处境和写这条时的处境不一样。能迁移的是权衡，不是当时那个结论。",
		},
		Before: Text{
			En: "Use the nginx front end. It is the right choice for a box that already serves a site.",
			Zh: "用 nginx 那套前端。机器上已经有站点的话，这是正确选择。",
		},
		After: Text{
			En: "Caddy owns port 443 and needs the box to have nothing else on it. One extra nginx site shares a box that already serves something, and costs you its certificate and reload path.",
			Zh: "Caddy 会独占 443，要求这台机器上没有别的站点。多加一份 nginx 站点可以和已有站点共存，代价是证书和 reload 都要自己管。",
		},
	},
	{
		ID:   "read-or-scan",
		Tier: TierJudge,
		What: Text{
			En: "Ask whether this is read or scanned. A lesson is prose; an inventory is a table.",
			Zh: "先想清楚这段是拿来读的还是拿来扫的。经验写成段落，清单做成表格。",
		},
		Why: Text{
			En: "A lesson forced into a table loses the reason, which is the part that transfers. An inventory written as prose cannot be looked up.",
			Zh: "经验塞进表格，丢掉的是原因，而原因才是能迁移的部分。清单写成散文，则没法查。",
		},
		Before: Text{
			En: "A table with the columns Symptom | Cause | Fix, one row, each cell a sentence fragment.",
			Zh: "一张「症状 | 原因 | 做法」的表，只有一行，每格都是半句话。",
		},
		After: Text{
			En: "Two sentences of prose for the one lesson; a table when there are eight servers to compare.",
			Zh: "一条经验就写两句话；八台服务器要对照的时候再上表格。",
		},
	},
	{
		ID:   "no-inflation",
		Tier: TierJudge,
		What: Text{
			En: "Do not tell the reader that something is important. State it and let it be.",
			Zh: "不要告诉读者「这件事很重要」。把它说出来就行了。",
		},
		Why: Text{
			En: "Marks it a milestone, underscores the importance, reflects a broader pattern: each is a sentence that adds weight and no fact.",
			Zh: "「这标志着」「充分体现了」「反映出更深层的」，每一句都只加分量，不加事实。",
		},
		Before: Text{
			En: "This incident underscores the critical importance of verification and marks a turning point in how the fleet is supervised.",
			Zh: "这次事故充分说明了验证的极端重要性，也标志着舰队监督方式的一个转折点。",
		},
		After: Text{
			En: "The check was skipped and the bug shipped. Run it before the tag, not after.",
			Zh: "那次跳过了检查，bug 就跟着发出去了。这个检查要在打 tag 之前跑，不是之后。",
		},
	},
}

// Rules is the whole table, mechanical rules first.
func Rules() []Rule {
	out := make([]Rule, 0, len(rules)+len(judgements))
	out = append(out, rules...)
	out = append(out, judgements...)
	return out
}

// plainFinding is one lint check's aggregated detail for one entry.
type plainFinding struct{ check, detail string }

// checkTail is the one-line instruction appended to a check's detail, because the act the
// finding asks for differs: a voice finding is a sentence to rewrite, a title finding is a
// title to replace.
func checkTail(c string) string {
	if c == checkTitle {
		return " — the title gets one line; `gtmux knowledge style` has the rule and an example"
	}
	return " — say it plainly; facts, quotes, numbers and commands stay verbatim"
}

// mechanicalFindings runs every rule that carries a matcher and aggregates per check, with
// the tiering the table declares: one strong sighting is enough, weak ones need each other.
func mechanicalFindings(e entryProse) []plainFinding {
	strong, weak := map[string][]string{}, map[string][]string{}
	var order []string
	seen := map[string]bool{}
	for _, r := range rules {
		if r.find == nil {
			continue
		}
		d := r.find(e)
		if d == "" {
			continue
		}
		c := r.Check()
		if !seen[c] {
			seen[c] = true
			order = append(order, c)
		}
		if r.Tier == TierStrong {
			strong[c] = append(strong[c], d)
		} else {
			weak[c] = append(weak[c], d)
		}
	}
	var out []plainFinding
	for _, c := range order {
		s, w := strong[c], weak[c]
		if len(s) == 0 && len(w) < 2 {
			continue
		}
		out = append(out, plainFinding{c, strings.Join(append(s, w...), " · ") + checkTail(c)})
	}
	return out
}

// ruleOut is one rule as `gtmux knowledge style --json` gives it: the table plus the two
// things a caller would otherwise have to derive.
type ruleOut struct {
	N          int    `json:"n"`
	ID         string `json:"id"`
	Tier       string `json:"tier"`
	Check      string `json:"check,omitempty"`
	Mechanical bool   `json:"mechanical"`
	What       Text   `json:"what"`
	Why        Text   `json:"why"`
	Before     Text   `json:"before"`
	After      Text   `json:"after"`
}

// RulesJSON is the table for an agent that wants it structured.
func RulesJSON() any {
	all := Rules()
	out := make([]ruleOut, 0, len(all))
	for i, r := range all {
		o := ruleOut{N: i + 1, ID: r.ID, Tier: r.Tier, Mechanical: r.Mechanical(),
			What: r.What, Why: r.Why, Before: r.Before, After: r.After}
		if o.Mechanical {
			o.Check = r.Check()
		}
		out = append(out, o)
	}
	return struct {
		Rules []ruleOut `json:"rules"`
	}{out}
}

// StyleText renders the table for a reader, in their language.
func StyleText(zh bool) string {
	pickText := func(t Text) string {
		if zh {
			return t.Zh
		}
		return t.En
	}
	var b strings.Builder
	all := Rules()
	if zh {
		fmt.Fprintf(&b, "知识库条目的写法 · %d 条\n", len(all))
		b.WriteString("规则只有这一份：lint 取其中能机器判的几条，HQ 章程指向这里。\n")
		b.WriteString("能执行、能核对的东西逐字不动；改的是说法，不是事实。\n")
	} else {
		fmt.Fprintf(&b, "How a knowledge entry reads · %d rules\n", len(all))
		b.WriteString("One copy: the lint takes the mechanical ones, the charter points here.\n")
		b.WriteString("Everything executable or checkable stays verbatim; the rewrite is of the saying.\n")
	}
	section := ""
	for i, r := range all {
		var want string
		switch r.Tier {
		case TierStrong:
			want = i18n.Tr("checked · one sighting is enough", "机器判 · 一次即算")
		case TierWeak:
			want = i18n.Tr("checked · counts alongside another", "机器判 · 要和别的一起出现才算")
		default:
			want = i18n.Tr("yours to judge · no check claims this one", "只能人判 · 没有检查会替你判这条")
		}
		if want != section {
			section = want
			b.WriteString("\n" + want + "\n")
		}
		fmt.Fprintf(&b, "\n%2d. %s\n", i+1, pickText(r.What))
		fmt.Fprintf(&b, "    %s %s\n", i18n.Tr("why", "为何"), pickText(r.Why))
		block := func(label, text string) {
			for i, ln := range strings.Split(text, "\n") {
				lead := label
				if i > 0 {
					lead = i18n.Tr("   ", "    ")
				}
				fmt.Fprintf(&b, "    %s %s\n", lead, ln)
			}
		}
		block(i18n.Tr("was", "改前"), pickText(r.Before))
		block(i18n.Tr("say", "改后"), pickText(r.After))
	}
	return b.String()
}
